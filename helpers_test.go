package stun

import (
	"errors"
	"testing"

	"github.com/gortc/stun/internal/testutil"
)

func BenchmarkBuildOverhead(b *testing.B) {
	var (
		t        = BindingRequest
		username = NewUsername("username")
		nonce    = NewNonce("nonce")
		realm    = NewRealm("example.org")
	)
	b.Run("Build", func(b *testing.B) {
		b.ReportAllocs()
		m := new(Message)
		for i := 0; i < b.N; i++ {
			m.Build(&t, &username, &nonce, &realm, &Fingerprint)
		}
	})
	b.Run("BuildNonPointer", func(b *testing.B) {
		b.ReportAllocs()
		m := new(Message)
		for i := 0; i < b.N; i++ {
			m.Build(t, username, nonce, realm, Fingerprint)
		}
	})
	b.Run("Raw", func(b *testing.B) {
		b.ReportAllocs()
		m := new(Message)
		for i := 0; i < b.N; i++ {
			m.Reset()
			m.WriteHeader()
			m.SetType(t)
			username.AddTo(m)
			nonce.AddTo(m)
			realm.AddTo(m)
			Fingerprint.AddTo(m)
		}
	})
}

func TestMessage_Apply(t *testing.T) {
	var (
		integrity = NewShortTermIntegrity("password")
		decoded   = new(Message)
	)
	m, err := Build(BindingRequest, TransactionID,
		NewUsername("username"),
		NewNonce("nonce"),
		NewRealm("example.org"),
		integrity,
		Fingerprint,
	)
	if err != nil {
		t.Fatal("failed to build:", err)
	}
	if m.Check(Fingerprint, integrity); err != nil {
		t.Fatal(err)
	}
	if _, err := decoded.Write(m.Raw); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(m) {
		t.Error("not equal")
	}
	if err := integrity.Check(decoded); err != nil {
		t.Fatal(err)
	}
}

type errReturner struct {
	Err error
}

func (e errReturner) AddTo(m *Message) error {
	return e.Err
}

func (e errReturner) Check(m *Message) error {
	return e.Err
}

func (e errReturner) GetFrom(m *Message) error {
	return e.Err
}

func TestHelpersErrorHandling(t *testing.T) {
	m := New()
	e := errReturner{Err: errors.New("tError")}
	if err := m.Build(e); err != e.Err {
		t.Error(err, "!=", e.Err)
	}
	if err := m.Check(e); err != e.Err {
		t.Error(err, "!=", e.Err)
	}
	if err := m.Parse(e); err != e.Err {
		t.Error(err, "!=", e.Err)
	}
	t.Run("MustBuild", func(t *testing.T) {
		t.Run("Positive", func(t *testing.T) {
			MustBuild(NewTransactionIDSetter(transactionID{}))
		})
		defer func() {
			if p := recover(); p != e.Err {
				t.Errorf("%s != %s",
					p, e.Err,
				)
			}
		}()
		MustBuild(e)
	})
}

func TestMessage_ForEach(t *testing.T) {
	initial := New()
	if err := initial.Build(
		NewRealm("realm1"), NewRealm("realm2"),
	); err != nil {
		t.Fatal(err)
	}
	newMessage := func() *Message {
		m := New()
		if err := m.Build(
			NewRealm("realm1"), NewRealm("realm2"),
		); err != nil {
			t.Fatal(err)
		}
		return m
	}
	t.Run("NoResults", func(t *testing.T) {
		m := newMessage()
		if !m.Equal(initial) {
			t.Error("m should be equal to initial")
		}
		if err := m.ForEach(AttrUsername, func(m *Message) error {
			t.Error("should not be called")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if !m.Equal(initial) {
			t.Error("m should be equal to initial")
		}
	})
	t.Run("ReturnOnError", func(t *testing.T) {
		m := newMessage()
		var calls int
		if err := m.ForEach(AttrRealm, func(m *Message) error {
			if calls > 0 {
				t.Error("called multiple times")
			}
			calls++
			return ErrAttributeNotFound
		}); err != ErrAttributeNotFound {
			t.Fatal(err)
		}
		if !m.Equal(initial) {
			t.Error("m should be equal to initial")
		}
	})
	t.Run("Positive", func(t *testing.T) {
		m := newMessage()
		var realms []string
		if err := m.ForEach(AttrRealm, func(m *Message) error {
			var realm Realm
			if err := realm.GetFrom(m); err != nil {
				return err
			}
			realms = append(realms, realm.String())
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if len(realms) != 2 {
			t.Fatal("expected 2 realms")
		}
		if realms[0] != "realm1" {
			t.Error("bad value for 1 realm")
		}
		if realms[1] != "realm2" {
			t.Error("bad value for 2 realm")
		}
		if !m.Equal(initial) {
			t.Error("m should be equal to initial")
		}
		t.Run("ZeroAlloc", func(t *testing.T) {
			m = newMessage()
			var realm Realm
			testutil.ShouldNotAllocate(t, func() {
				if err := m.ForEach(AttrRealm, func(m *Message) error {
					if err := realm.GetFrom(m); err != nil {
						return err
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			})
			if !m.Equal(initial) {
				t.Error("m should be equal to initial")
			}
		})
	})
}
