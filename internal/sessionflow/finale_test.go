package sessionflow

import (
	"gddoom/internal/mapdata"
	"testing"
)

func TestStartFinaleEpisodeOneStartsWithTextStage(t *testing.T) {
	state, ok := StartFinale("E1M8", false)
	if !ok {
		t.Fatal("StartFinale(E1M8)=false want true")
	}
	if state.Stage != FinaleStageText {
		t.Fatalf("stage=%d want %d", state.Stage, FinaleStageText)
	}
	if state.Flat != "FLOOR4_8" {
		t.Fatalf("flat=%q want FLOOR4_8", state.Flat)
	}
	if state.Screen != "CREDIT" {
		t.Fatalf("screen=%q want CREDIT", state.Screen)
	}
	if state.Text != e1Text {
		t.Fatal("unexpected E1 finale text")
	}
}

func TestTickFinaleTransitionsFromTextToPicture(t *testing.T) {
	state, ok := StartFinale("E1M8", false)
	if !ok {
		t.Fatal("StartFinale(E1M8)=false want true")
	}
	state.Tic = finaleTextTotalTics(state.Text)
	next, done := TickFinale(state, false)
	if done {
		t.Fatal("TickFinale() done=true want false on text->picture transition")
	}
	if next.Stage != FinaleStagePicture {
		t.Fatalf("stage=%d want %d", next.Stage, FinaleStagePicture)
	}
	if next.Tic != 0 {
		t.Fatalf("tic=%d want 0 after text->picture transition", next.Tic)
	}
}

func TestFinaleVisibleTextUsesDoomTiming(t *testing.T) {
	if got := FinaleVisibleText("ABC", finaleTextStartDelay-1); got != "" {
		t.Fatalf("visible text before delay=%q want empty", got)
	}
	if got := FinaleVisibleText("ABC", finaleTextStartDelay+FinaleTextSpeed); got != "A" {
		t.Fatalf("visible text after first interval=%q want A", got)
	}
	if got := FinaleVisibleText("ABC", finaleTextStartDelay+FinaleTextSpeed*4); got != "ABC" {
		t.Fatalf("visible text after full reveal=%q want ABC", got)
	}
}

func TestCommercialFinaleSelectionAndHeldButtonTiming(t *testing.T) {
	for _, tc := range []struct {
		name         mapdata.MapName
		secret, want bool
	}{
		{"MAP06", false, true}, {"MAP11", false, true}, {"MAP20", false, true},
		{"MAP30", false, true}, {"MAP15", false, false}, {"MAP15", true, true},
		{"MAP31", false, false}, {"MAP31", true, true}, {"MAP07", false, false},
	} {
		state, ok := StartCommercialFinale(tc.name, tc.secret)
		if ok != tc.want {
			t.Fatalf("%s secret=%t finale=%t want=%t", tc.name, tc.secret, ok, tc.want)
		}
		if !ok {
			continue
		}
		if !state.Commercial || state.Text == "" || state.Flat == "" {
			t.Fatalf("incomplete finale: %+v", state)
		}
		for i := 0; i < 51; i++ {
			var done bool
			state, done = TickFinale(state, true)
			if done || state.Stage != FinaleStageText {
				t.Fatalf("%s transitioned before finalecount>50", tc.name)
			}
		}
		next, done := TickFinale(state, true)
		if tc.name == "MAP30" {
			if done || next.Stage != FinaleStageCast {
				t.Fatal("MAP30 must enter cast without loading a level")
			}
		} else if !done {
			t.Fatalf("%s did not queue worlddone on the 52nd held-button tic", tc.name)
		}
		state.Tic = 100000
		next, done = TickFinale(state, false)
		if done || next.Stage != FinaleStageText {
			t.Fatal("commercial text must wait indefinitely without a button")
		}
	}
}

func TestBoomCommercialFinaleAccelerationAndTimeout(t *testing.T) {
	state, _ := StartCommercialFinale("MAP15", true)
	state.Boom = true
	state.Tic = 20
	state, done := TickFinale(state, true)
	if done || !state.Accelerated || state.VisibleText() != state.Text {
		t.Fatal("first edge should accelerate early text without leaving")
	}
	if _, done := TickFinale(state, false); done {
		t.Fatal("held button was treated as another edge")
	}
	if _, done := TickFinale(state, true); !done {
		t.Fatal("second edge did not queue worlddone")
	}
	cast, _ := StartCommercialFinale("MAP30", false)
	cast.Boom, cast.Accelerated = true, true
	if next, done := TickFinale(cast, true); done || next.Stage != FinaleStageCast {
		t.Fatal("MAP30 skipped its cast")
	}
	state, _ = StartCommercialFinale("MAP15", true)
	state.Boom, state.Text = true, "TEST"
	state, _ = TickFinale(state, true)
	for state.Tic < 1000 {
		state, done = TickFinale(state, false)
		if done {
			t.Fatal("accelerated finale timed out too early")
		}
	}
	if _, done := TickFinale(state, false); !done {
		t.Fatal("accelerated text did not time out")
	}
	state.Accelerated, state.Tic = false, 100000
	if _, done := TickFinale(state, false); done {
		t.Fatal("unaccelerated commercial text should wait for input")
	}
}

func TestBoomEpisodeFinaleStaysOnArtAndDoesNotConsumeCastInput(t *testing.T) {
	f, _ := StartFinale("E3M8", false)
	f.Boom = true
	f, _ = TickFinale(f, true)
	if !f.Accelerated || f.Stage != FinaleStageText {
		t.Fatal("first edge must reveal text")
	}
	f, _ = TickFinale(f, false)
	f, _ = TickFinale(f, true)
	if f.Stage != FinaleStagePicture || f.Tic != 0 {
		t.Fatal("second edge must start art")
	}
	for range 1500 {
		var done bool
		f, done = TickFinale(f, true)
		if done || !f.Active || f.Stage != FinaleStagePicture {
			t.Fatal("Boom art returned to frontend")
		}
	}
}

func TestCastKeysAndFullRosterCycleDoNotRunWorldActions(t *testing.T) {
	f, _ := StartCommercialFinale("MAP30", false)
	f.Boom = true
	f.Accelerated = true
	f, _ = TickFinale(f, true)
	first := f.Cast
	for range 200 {
		f, _ = TickFinale(f, true)
	}
	if f.Cast.Death || f.Cast.Actor != 0 {
		t.Fatal("recorded fire/use killed cast actor")
	}
	if first.State == f.Cast.State && first.Tics == f.Cast.Tics && first.Frames == f.Cast.Frames {
		t.Fatal("cast did not animate")
	}
	for actor := 0; actor < len(castActors); actor++ {
		if f.Cast.Actor != actor {
			t.Fatalf("actor=%d want %d", f.Cast.Actor, actor)
		}
		f.KillCast()
		if !f.Cast.Death || f.Cast.Attacking {
			t.Fatal("host key did not trigger death")
		}
		for ticks := 0; f.Cast.Actor == actor && ticks < 300; ticks++ {
			f, _ = TickFinale(f, false)
		}
	}
	if f.Cast.Actor != 0 || f.Cast.Death {
		t.Fatal("cast did not wrap after the hero")
	}
}
