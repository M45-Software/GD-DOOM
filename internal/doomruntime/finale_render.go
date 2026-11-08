package doomruntime

import (
	"fmt"
	"strings"

	"gddoom/internal/render/levelmesh"
	"gddoom/internal/sessionflow"
	"github.com/hajimehoshi/ebiten/v2"
)

func (sg *sessionGame) drawCastFinale(screen *ebiten.Image, scale, ox, oy float64) {
	sg.drawIntermissionPatch(screen, "BOSSBACK", 0, 0, scale, ox, oy, false)
	c := sg.finale.Cast
	if !c.Valid() || sg.g == nil {
		return
	}
	sg.drawIntermissionText(screen, c.Name(), 160, 180, scale, ox, oy, true)
	prefix, frame := c.Sprite()
	name := spriteFrameName(prefix, frame, '0')
	flip := false
	if !sg.g.spritePatchExists(name) {
		var ok bool
		name, flip, ok = sg.g.monsterSpriteRotFrame(prefix, frame, 1)
		if !ok {
			return
		}
	}
	patch, ok := sg.g.monsterSpriteTexture(name)
	if !ok {
		return
	}
	p := *patch
	x := 160 - p.OffsetX
	if flip {
		x = 160 + p.OffsetX - p.Width
	}
	y := 170 - p.OffsetY
	if sg.nativePatches != nil {
		if flip {
			p = flipFinalePatch(p)
		}
		*sg.nativePatches = append(*sg.nativePatches, levelmesh.Patch{Texture: nativeTexture(&p), X: ox + float64(x)*scale, Y: oy + float64(y)*scale, W: float64(p.Width) * scale, H: float64(p.Height) * scale})
		return
	}
	img, ok := sg.cachedPatchImage(&sg.spritePatchImages, name, p)
	if !ok || screen == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	if flip {
		op.GeoM.Scale(-1, 1)
		op.GeoM.Translate(float64(p.Width), 0)
	}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(ox+float64(x)*scale, oy+float64(y)*scale)
	screen.DrawImage(img, op)
}
func flipFinalePatch(p WallTexture) WallTexture {
	rgba := make([]byte, len(p.RGBA))
	for y := 0; y < p.Height; y++ {
		for x := 0; x < p.Width; x++ {
			to := (y*p.Width + x) * 4
			from := (y*p.Width + p.Width - 1 - x) * 4
			if from+4 <= len(p.RGBA) {
				copy(rgba[to:to+4], p.RGBA[from:from+4])
			}
		}
	}
	p.RGBA = rgba
	return p
}

func (sg *sessionGame) drawEpisodeFinalePicture(screen *ebiten.Image, scale, ox, oy float64) {
	f := sg.finale
	if f.Boom && strings.HasPrefix(string(f.MapName), "E3") {
		scrolled := 320 - (f.Tic-230)/2
		scrolled = max(0, min(320, scrolled))
		sg.drawIntermissionPatch(screen, "PFUB1", 320-scrolled, 0, scale, ox, oy, false)
		sg.drawIntermissionPatch(screen, "PFUB2", -scrolled, 0, scale, ox, oy, false)
		if f.Tic >= 1130 {
			stage := 0
			if f.Tic >= 1180 {
				stage = min(6, (f.Tic-1180)/5)
			}
			sg.drawIntermissionPatch(screen, fmt.Sprintf("END%d", stage), 108, 68, scale, ox, oy, false)
		}
		return
	}
	sg.drawIntermissionPatch(screen, f.Screen, 0, 0, scale, ox, oy, false)
	if !f.Boom && (f.Tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO CONTINUE", 160, 186, scale, ox, oy, true)
	}
}

func (sg *sessionGame) playFinaleCue(cue string) {
	if cue == "" || cue == "None" || sg.g == nil || sg.g.snd == nil {
		return
	}
	for event := soundEvent(1); event < soundEventCount; event++ {
		if name, ok := soundEventDSName(event); ok && strings.EqualFold(name, "DS"+cue) {
			sg.g.snd.playEvent(event)
			return
		}
	}
}

func (sg *sessionGame) playFinaleMusicLump(lump string) bool {
	if sg.musicCtl == nil || sg.opts.MusicPlayerTrackLoader == nil {
		return false
	}
	track, key := sg.findCurrentCatalogTrackByLump(lump)
	if track == nil {
		return false
	}
	data, err := sg.opts.MusicPlayerTrackLoader(key, track.LumpName)
	if err != nil || len(data) == 0 {
		return false
	}
	sg.musicCtl.PlayData(data, effectiveMusicPlaybackVolume(sg.opts))
	sg.currentMusicSource = musicPlaybackSource{kind: musicPlaybackSourcePlayer, wadKey: key, lumpName: lump, musicName: track.MusicName}
	sg.setNowPlayingMusic(track.MusicName, lump)
	return true
}

func (sg *sessionGame) finaleEnteredStage(previous sessionflow.FinaleStage) {
	if sg.finale.Stage != previous {
		if sg.finale.Stage == sessionflow.FinaleStageCast {
			if !sg.playFinaleMusicLump("D_EVIL") && sg.musicCtl != nil && sg.opts.MapMusicLoader != nil {
				if parsed, err := sg.opts.MapMusicLoader("MAP31"); err == nil && parsed != nil {
					sg.musicCtl.PlayParsed(parsed, effectiveMusicPlaybackVolume(sg.opts))
					sg.setNowPlayingMusic("Cast of Characters")
				}
			}
		} else if sg.finale.Stage == sessionflow.FinaleStagePicture && strings.HasPrefix(string(sg.finale.MapName), "E3") {
			sg.playFinaleMusicLump("D_BUNNY")
		}
	}
	f := sg.finale
	if f.Boom && f.Stage == sessionflow.FinaleStagePicture && strings.HasPrefix(string(f.MapName), "E3") && f.Tic >= 1185 && f.Tic <= 1210 && (f.Tic-1180)%5 == 0 {
		sg.playFinaleCue("pistol")
	}
}

// Resume the appropriate score after a recorded-session restore. Playback
// restarts the track; sample-position audio seeking is outside checkpoint state.
func (sg *sessionGame) resumeFinaleCheckpointMusic() {
	if sg.finale.Active {
		f := sg.finale
		switch {
		case f.Stage == sessionflow.FinaleStageCast:
			sg.playFinaleMusicLump("D_EVIL")
		case f.Boom && f.Stage == sessionflow.FinaleStagePicture && strings.HasPrefix(string(f.MapName), "E3"):
			sg.playFinaleMusicLump("D_BUNNY")
		case f.Commercial:
			sg.playFinaleMusicLump("D_READ_M")
		default:
			if sg.musicCtl != nil && sg.opts.FinaleMusicLoader != nil {
				if parsed, err := sg.opts.FinaleMusicLoader(string(sg.current), false); err == nil && parsed != nil {
					sg.musicCtl.PlayParsed(parsed, effectiveMusicPlaybackVolume(sg.opts))
				}
			}
		}
	} else if sg.intermission.state.Active {
		sg.playIntermissionMusic(sg.intermission.state.Commercial)
	}
}
