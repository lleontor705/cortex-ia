package pipeline

import (
	"errors"
	"fmt"
	"testing"
)

type mockStep struct {
	name       string
	err        error
	ran        bool
	rolledBack bool
}

func (s *mockStep) Name() string    { return s.name }
func (s *mockStep) Run() error      { s.ran = true; return s.err }
func (s *mockStep) Rollback() error { s.rolledBack = true; return nil }

type funcStep struct {
	name string
	run  func() error
}

func (s funcStep) Name() string { return s.name }
func (s funcStep) Run() error   { return s.run() }

func TestRunStage_AllSuccess(t *testing.T) {
	s1 := &mockStep{name: "step1"}
	s2 := &mockStep{name: "step2"}
	s3 := &mockStep{name: "step3"}

	result := RunStage([]Step{s1, s2, s3})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if len(result.Completed) != 3 {
		t.Errorf("expected 3 completed, got %d", len(result.Completed))
	}
}

func TestRunStage_FailureRollsBack(t *testing.T) {
	s1 := &mockStep{name: "step1"}
	s2 := &mockStep{name: "step2", err: errors.New("boom")}
	s3 := &mockStep{name: "step3"}

	result := RunStage([]Step{s1, s2, s3})
	if result.Error == nil {
		t.Error("expected error")
	}
	if result.Failed != "step2" {
		t.Errorf("expected step2 to fail, got %s", result.Failed)
	}
	if !s1.rolledBack {
		t.Error("expected step1 to be rolled back")
	}
	if s3.ran {
		t.Error("step3 should not have run")
	}
}

func TestRunStage_Empty(t *testing.T) {
	result := RunStage(nil)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
}

func TestRunParallelChains_StopsChainAfterFirstError(t *testing.T) {
	var ran []string
	record := func(name string, err error) Step {
		return funcStep{name: name, run: func() error {
			ran = append(ran, name)
			return err
		}}
	}

	result := RunParallelChains([][]Step{{
		record("first", nil),
		record("failed", errors.New("boom")),
		record("after-failure", nil),
	}})

	if got, want := fmt.Sprint(ran), "[first failed]"; got != want {
		t.Errorf("ran = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(result.Completed), "[first]"; got != want {
		t.Errorf("completed = %s, want %s", got, want)
	}
	if result.Error == nil {
		t.Fatal("expected an error")
	}
}

func TestRunParallelChains_OverlapsChainsWaitsForPeersAndOrdersErrors(t *testing.T) {
	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	peerSecondStarted := make(chan struct{})
	releasePeer := make(chan struct{})
	resultCh := make(chan StageResult, 1)

	first := func(name string, err error) Step {
		return funcStep{name: name, run: func() error {
			started <- name
			<-releaseFirst
			return err
		}}
	}
	peerSecond := funcStep{name: "peer-second", run: func() error {
		close(peerSecondStarted)
		<-releasePeer
		return nil
	}}

	go func() {
		resultCh <- RunParallelChains([][]Step{
			{first("chain-one-failure", errors.New("first chain"))},
			{first("chain-two-first", nil), peerSecond},
			{funcStep{name: "chain-three-failure", run: func() error { return errors.New("third chain") }}},
		})
	}()

	for range 2 {
		<-started
	}
	close(releaseFirst)
	<-peerSecondStarted
	select {
	case result := <-resultCh:
		t.Fatalf("returned before running peer completed: %+v", result)
	default:
	}
	close(releasePeer)

	result := <-resultCh
	if got, want := fmt.Sprint(result.Completed), "[chain-two-first peer-second]"; got != want {
		t.Errorf("completed = %s, want %s", got, want)
	}
	if result.Error == nil {
		t.Fatal("expected an error")
	}
	if got, want := result.Error.Error(), `2 step(s) failed: step "chain-one-failure": first chain`; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}
