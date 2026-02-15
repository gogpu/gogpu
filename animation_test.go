package gogpu

import (
	"sync"
	"testing"
)

func TestAnimationController_NoAnimations(t *testing.T) {
	ac := &AnimationController{}
	if ac.IsAnimating() {
		t.Error("expected IsAnimating=false for new controller, got true")
	}
}

func TestAnimationController_StartStop(t *testing.T) {
	ac := &AnimationController{}

	token := ac.StartAnimation()
	if !ac.IsAnimating() {
		t.Error("expected IsAnimating=true after StartAnimation, got false")
	}

	token.Stop()
	if ac.IsAnimating() {
		t.Error("expected IsAnimating=false after Stop, got true")
	}
}

func TestAnimationController_MultipleAnimations(t *testing.T) {
	ac := &AnimationController{}

	t1 := ac.StartAnimation()
	t2 := ac.StartAnimation()
	t3 := ac.StartAnimation()

	if !ac.IsAnimating() {
		t.Error("expected IsAnimating=true with 3 active animations")
	}

	t1.Stop()
	if !ac.IsAnimating() {
		t.Error("expected IsAnimating=true with 2 active animations")
	}

	t2.Stop()
	if !ac.IsAnimating() {
		t.Error("expected IsAnimating=true with 1 active animation")
	}

	t3.Stop()
	if ac.IsAnimating() {
		t.Error("expected IsAnimating=false after all animations stopped")
	}
}

func TestAnimationToken_DoubleStop(t *testing.T) {
	ac := &AnimationController{}

	token := ac.StartAnimation()
	token.Stop()
	token.Stop() // second call must be a no-op

	if ac.IsAnimating() {
		t.Error("expected IsAnimating=false after double Stop")
	}

	// Verify count did not underflow: start another animation,
	// count should go from 0 to 1 (not from -1 to 0).
	t2 := ac.StartAnimation()
	if !ac.IsAnimating() {
		t.Error("expected IsAnimating=true after new StartAnimation")
	}
	t2.Stop()
	if ac.IsAnimating() {
		t.Error("expected IsAnimating=false after stopping second animation")
	}
}

func TestAnimationController_ConcurrentAccess(t *testing.T) {
	ac := &AnimationController{}
	const goroutines = 100

	var wg sync.WaitGroup
	tokens := make([]*AnimationToken, goroutines)

	// Start all animations concurrently.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			tokens[idx] = ac.StartAnimation()
		}(i)
	}
	wg.Wait()

	if !ac.IsAnimating() {
		t.Errorf("expected IsAnimating=true with %d active animations", goroutines)
	}

	// Stop all animations concurrently.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			tokens[idx].Stop()
		}(i)
	}
	wg.Wait()

	if ac.IsAnimating() {
		t.Error("expected IsAnimating=false after all concurrent stops")
	}
}
