package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// mprisService owns the D-Bus connection and exported properties. It does
// not read audioPlayer's state directly — it's kept in sync entirely by
// feeding it a PlayerStatus subscription via run().
type mprisService struct {
	conn  *dbus.Conn
	props *prop.Properties

	lastStatus PlayerStatus
}

type trackMeta struct {
	title  string
	artist string
	album  string
	artURL string
}

func readTrackMeta(filePath string) trackMeta {
	meta := trackMeta{title: strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))}

	f, err := os.Open(filePath)
	if err != nil {
		return meta
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return meta
	}
	if t := m.Title(); t != "" {
		meta.title = t
	}
	meta.artist = m.Artist()
	meta.album = m.Album()

	if pic := m.Picture(); pic != nil {
		ext := ".jpg"
		if strings.Contains(pic.MIMEType, "png") {
			ext = ".png"
		}
		artPath := filepath.Join(os.TempDir(), fmt.Sprintf("sawnd-cover-%d%s", os.Getpid(), ext))
		if err := os.WriteFile(artPath, pic.Data, 0o644); err == nil {
			meta.artURL = "file://" + artPath
		}
	}
	return meta
}

type mprisRoot struct{}

func (r *mprisRoot) Raise() *dbus.Error { return nil }
func (r *mprisRoot) Quit() *dbus.Error  { return nil }

// mprisPlayer handles incoming MPRIS *commands*. This direction (external
// controller -> audioPlayer) is unrelated to status flow and still calls
// into ap directly, same as the TUI's key handlers do.
type mprisPlayer struct {
	ap *audioPlayer
}

func (p *mprisPlayer) Play() *dbus.Error {
	p.ap.conn.Set("pause", false)
	return nil
}
func (p *mprisPlayer) Pause() *dbus.Error {
	p.ap.conn.Set("pause", true)
	return nil
}
func (p *mprisPlayer) PlayPause() *dbus.Error {
	p.ap.togglePause()
	return nil
}
func (p *mprisPlayer) Stop() *dbus.Error {
	p.ap.conn.Set("pause", true)
	return nil
}
func (p *mprisPlayer) Next() *dbus.Error     { return nil }
func (p *mprisPlayer) Previous() *dbus.Error { return nil }
func (p *mprisPlayer) Seek(offsetUs int64) *dbus.Error {
	p.ap.seek(int(offsetUs / 1_000_000))
	return nil
}
func (p *mprisPlayer) SetPosition(trackID dbus.ObjectPath, positionUs int64) *dbus.Error {
	target := time.Duration(positionUs) * time.Microsecond
	delta := target - p.ap.Position()
	p.ap.seek(int(delta.Seconds()))
	return nil
}
func (p *mprisPlayer) OpenUri(uri string) *dbus.Error { return nil }

// startMPRIS registers sawnd as its own MPRIS2 player and returns a
// service. It does NOT start reading status on its own — call run() with
// a subscription from ap.Subscribe() to keep it live.
func startMPRIS(ap *audioPlayer, filePath string) (*mprisService, error) {
	meta := readTrackMeta(filePath)

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect session bus: %w", err)
	}

	reply, err := conn.RequestName("org.mpris.MediaPlayer2.sawnd", dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil, fmt.Errorf("could not own MPRIS bus name (already running?)")
	}

	conn.Export(&mprisRoot{}, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2")
	conn.Export(&mprisPlayer{ap: ap}, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player")

	trackLengthUs := ap.length
	metadata := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath("/org/sawnd/track1")),
		"xesam:title":   dbus.MakeVariant(meta.title),
		"mpris:length":  dbus.MakeVariant(trackLengthUs),
	}
	// if length, lenErr := ap.conn.Get("duration"); lenErr == nil {
	// 	if v, ok := length.(float64); ok {
	// 		dur := time.Duration(v * float64(time.Second))
	// 		// FIX: Pass explicitly as microseconds (int64)
	// 		metadata["mpris:length"] = dbus.MakeVariant(dur.Microseconds())
	// 	}
	// }
	if meta.artist != "" {
		metadata["xesam:artist"] = dbus.MakeVariant([]string{meta.artist})
	}
	if meta.album != "" {
		metadata["xesam:album"] = dbus.MakeVariant(meta.album)
	}
	if meta.artURL != "" {
		metadata["mpris:artUrl"] = dbus.MakeVariant(meta.artURL)
	}

	propsSpec := map[string]map[string]*prop.Prop{
		"org.mpris.MediaPlayer2": {
			"CanQuit":      {Value: false, Writable: false, Emit: prop.EmitTrue},
			"CanRaise":     {Value: false, Writable: false, Emit: prop.EmitTrue},
			"Identity":     {Value: "sawnd", Writable: false, Emit: prop.EmitTrue},
			"HasTrackList": {Value: false, Writable: false, Emit: prop.EmitTrue},
		},
		"org.mpris.MediaPlayer2.Player": {
			"PlaybackStatus": {Value: "Playing", Writable: false, Emit: prop.EmitTrue},
			"Metadata":       {Value: metadata, Writable: false, Emit: prop.EmitTrue},
			"Position":       {Value: int64(0), Writable: false, Emit: prop.EmitFalse},
			"Volume": {
				Value: float64(ap.volume()) / 100.0, Writable: true, Emit: prop.EmitTrue,
				Callback: func(c *prop.Change) *dbus.Error {
					v, ok := c.Value.(float64)
					if !ok {
						return nil
					}
					pct := clamp(int(v*100), 0, 100)
					ap.mu.Lock()
					ap.volumePct = pct
					ap.mu.Unlock()
					ap.conn.Set("volume", pct)
					return nil
				},
			},
			"CanGoNext":     {Value: false, Writable: false, Emit: prop.EmitTrue},
			"CanGoPrevious": {Value: false, Writable: false, Emit: prop.EmitTrue},
			"CanPlay":       {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanPause":      {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanSeek":       {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanControl":    {Value: true, Writable: false, Emit: prop.EmitTrue},
		},
	}

	props, err := prop.Export(conn, "/org/mpris/MediaPlayer2", propsSpec)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("export properties: %w", err)
	}

	node := &introspect.Node{
		Name: "/org/mpris/MediaPlayer2",
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
		},
	}
	conn.Export(introspect.NewIntrospectable(node), "/org/mpris/MediaPlayer2", "org.freedesktop.DBus.Introspectable")

	return &mprisService{conn: conn, props: props}, nil
}

// run consumes status updates until the channel is closed (i.e. until
// ap.close() is called). Meant to be launched in its own goroutine.
func (m *mprisService) run(updates <-chan PlayerStatus) {
	for s := range updates {
		if m.lastStatus == s {
			continue
		}
		// 1. If length wasn't set at startup, update it as soon as audioPlayer reports it!
		if m.lastStatus.Length != s.Length && s.Length > 0 {
			m.lastStatus.Length = s.Length
			// Fetch existing metadata variant map from D-Bus prop store
			meta, _ := m.props.Get("org.mpris.MediaPlayer2.Player", "Metadata")
			if currentMeta, ok := meta.Value().(map[string]dbus.Variant); ok {
				// Set the mpris:length in MICROSECONDS as int64
				currentMeta["mpris:length"] = dbus.MakeVariant(s.Length.Microseconds())

				// Push updated metadata to D-Bus (this triggers a PropertiesChanged signal for widgets!)
				m.props.SetMust("org.mpris.MediaPlayer2.Player", "Metadata", currentMeta)
			}
		}

		// 2. Playback State
		if s.State != m.lastStatus.State {
			var status string
			switch s.State {
			case StatePlaying:
				status = "Playing"
			case StatePaused:
				status = "Paused"
			case StateStopped:
				status = "Stopped"
			}
			m.props.SetMust("org.mpris.MediaPlayer2.Player", "PlaybackStatus", status)
			m.lastStatus.State = s.State
		}

		// 3. Position Update
		m.props.SetMust("org.mpris.MediaPlayer2.Player", "Position", s.Position.Microseconds())

		// 4. Volume Update
		if s.VolumePct != m.lastStatus.VolumePct {
			m.props.SetMust("org.mpris.MediaPlayer2.Player", "Volume", float64(s.VolumePct)/100.0)
			m.lastStatus.VolumePct = s.VolumePct
		}
	}
}

func (m *mprisService) Close() {
	if m.conn != nil {
		m.conn.ReleaseName("org.mpris.MediaPlayer2.sawnd")
		m.conn.Close()
	}
}
