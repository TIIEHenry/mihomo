package core

import (
	"reflect"
	"testing"
	"time"
)

func Test_fragUDPMessage(t *testing.T) {
	type args struct {
		m       udpMessage
		maxSize int
	}
	tests := []struct {
		name string
		args args
		want []udpMessage
	}{
		{
			"no frag",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    0,
					FragCount: 1,
					Data:      []byte("hello"),
				},
				100,
			},
			[]udpMessage{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    0,
					FragCount: 1,
					Data:      []byte("hello"),
				},
			},
		},
		{
			"2 frags",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    0,
					FragCount: 1,
					Data:      []byte("hello"),
				},
				22,
			},
			[]udpMessage{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    0,
					FragCount: 2,
					Data:      []byte("hell"),
				},
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    1,
					FragCount: 2,
					Data:      []byte("o"),
				},
			},
		},
		{
			"4 frags",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    0,
					FragCount: 1,
					Data:      []byte("wow wow wow lol lmao"),
				},
				23,
			},
			[]udpMessage{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    0,
					FragCount: 4,
					Data:      []byte("wow w"),
				},
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    1,
					FragCount: 4,
					Data:      []byte("ow wo"),
				},
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    2,
					FragCount: 4,
					Data:      []byte("w lol"),
				},
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    3,
					FragCount: 4,
					Data:      []byte(" lmao"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fragUDPMessage(tt.args.m, tt.args.maxSize); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fragUDPMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_defragger_Feed(t *testing.T) {
	d := &defragger{}
	type args struct {
		m udpMessage
	}
	tests := []struct {
		name string
		args args
		want *udpMessage
	}{
		{
			"no frag",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     123,
					FragID:    0,
					FragCount: 1,
					Data:      []byte("hello"),
				},
			},
			&udpMessage{
				SessionID: 123,
				Host:      "test",
				Port:      123,
				MsgID:     123,
				FragID:    0,
				FragCount: 1,
				Data:      []byte("hello"),
			},
		},
		{
			"frag 1 - 1/3",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     666,
					FragID:    0,
					FragCount: 3,
					Data:      []byte("hello"),
				},
			},
			nil,
		},
		{
			"frag 1 - 2/3",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     666,
					FragID:    1,
					FragCount: 3,
					Data:      []byte(" shitty "),
				},
			},
			nil,
		},
		{
			"frag 1 - 3/3",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     666,
					FragID:    2,
					FragCount: 3,
					Data:      []byte("world!!"),
				},
			},
			&udpMessage{
				SessionID: 123,
				Host:      "test",
				Port:      123,
				MsgID:     666,
				FragID:    0,
				FragCount: 1,
				Data:      []byte("hello shitty world!!"),
			},
		},
		{
			"frag A1",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     777,
					FragID:    0,
					FragCount: 2,
					Data:      []byte("hello"),
				},
			},
			nil,
		},
		{
			"frag B1",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     778,
					FragID:    0,
					FragCount: 2,
					Data:      []byte("hello"),
				},
			},
			nil,
		},
		{
			"frag A2",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     777,
					FragID:    1,
					FragCount: 2,
					Data:      []byte(" moto"),
				},
			},
			&udpMessage{
				SessionID: 123,
				Host:      "test",
				Port:      123,
				MsgID:     777,
				FragID:    0,
				FragCount: 1,
				Data:      []byte("hello moto"),
			},
		},
		{
			"frag B2",
			args{
				udpMessage{
					SessionID: 123,
					Host:      "test",
					Port:      123,
					MsgID:     778,
					FragID:    1,
					FragCount: 2,
					Data:      []byte(" moto"),
				},
			},
			&udpMessage{
				SessionID: 123,
				Host:      "test",
				Port:      123,
				MsgID:     778,
				FragID:    0,
				FragCount: 1,
				Data:      []byte("hello moto"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.Feed(tt.args.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Feed() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("interleaved sessions", func(t *testing.T) {
		sd := &defragger{}
		if got := sd.Feed(udpMessage{SessionID: 1, MsgID: 5, FragID: 0, FragCount: 2, Data: []byte("aa")}); got != nil {
			t.Fatalf("A1 = %v", got)
		}
		if got := sd.Feed(udpMessage{SessionID: 2, MsgID: 5, FragID: 0, FragCount: 2, Data: []byte("bb")}); got != nil {
			t.Fatalf("B1 = %v", got)
		}
		got := sd.Feed(udpMessage{SessionID: 1, MsgID: 5, FragID: 1, FragCount: 2, Data: []byte("AA")})
		if got == nil || string(got.Data) != "aaAA" || got.SessionID != 1 {
			t.Fatalf("A2 = %v", got)
		}
		got = sd.Feed(udpMessage{SessionID: 2, MsgID: 5, FragID: 1, FragCount: 2, Data: []byte("BB")})
		if got == nil || string(got.Data) != "bbBB" || got.SessionID != 2 {
			t.Fatalf("B2 = %v", got)
		}
	})

	t.Run("zero msgid", func(t *testing.T) {
		zd := &defragger{}
		got := zd.Feed(udpMessage{MsgID: 0, FragID: 0, FragCount: 2, Data: []byte("z")})
		if got != nil {
			t.Fatalf("Feed() = %v", got)
		}
	})

	t.Run("frag count replaces the slot", func(t *testing.T) {
		gd := &defragger{}
		if got := gd.Feed(udpMessage{SessionID: 1, MsgID: 2, FragID: 0, FragCount: 2, Data: []byte("a")}); got != nil {
			t.Fatalf("first = %v", got)
		}
		if got := gd.Feed(udpMessage{SessionID: 1, MsgID: 2, FragID: 0, FragCount: 3, Data: []byte("x")}); got != nil {
			t.Fatalf("replaced = %v", got)
		}
		if got := gd.Feed(udpMessage{SessionID: 1, MsgID: 2, FragID: 1, FragCount: 3, Data: []byte("y")}); got != nil {
			t.Fatalf("middle = %v", got)
		}
		got := gd.Feed(udpMessage{SessionID: 1, MsgID: 2, FragID: 2, FragCount: 3, Data: []byte("z")})
		if got == nil || string(got.Data) != "xyz" {
			t.Fatalf("assembled = %v", got)
		}
	})

	t.Run("msgid reuse drops the old payload", func(t *testing.T) {
		rd := &defragger{}
		if got := rd.Feed(udpMessage{SessionID: 1, MsgID: 9, FragID: 0, FragCount: 2, Data: []byte("a")}); got != nil {
			t.Fatalf("first = %v", got)
		}
		if got := rd.Feed(udpMessage{SessionID: 1, MsgID: 9, FragID: 0, FragCount: 2, Data: []byte("c")}); got != nil {
			t.Fatalf("reused = %v", got)
		}
		got := rd.Feed(udpMessage{SessionID: 1, MsgID: 9, FragID: 1, FragCount: 2, Data: []byte("d")})
		if got == nil || string(got.Data) != "cd" {
			t.Fatalf("assembled = %v", got)
		}
	})

	t.Run("duplicate fragment keeps the slot", func(t *testing.T) {
		dd := &defragger{}
		if got := dd.Feed(udpMessage{SessionID: 1, MsgID: 4, FragID: 0, FragCount: 2, Data: []byte("a")}); got != nil {
			t.Fatalf("first = %v", got)
		}
		if got := dd.Feed(udpMessage{SessionID: 1, MsgID: 4, FragID: 0, FragCount: 2, Data: []byte("a")}); got != nil {
			t.Fatalf("dup = %v", got)
		}
		got := dd.Feed(udpMessage{SessionID: 1, MsgID: 4, FragID: 1, FragCount: 2, Data: []byte("b")})
		if got == nil || string(got.Data) != "ab" {
			t.Fatalf("assembled = %v", got)
		}
	})

	t.Run("expired slot does not block the msgid", func(t *testing.T) {
		at := time.Unix(1_700_000_000, 0)
		ed := &defragger{now: func() time.Time { return at }}
		if got := ed.Feed(udpMessage{SessionID: 1, MsgID: 7, FragID: 0, FragCount: 2, Data: []byte("a")}); got != nil {
			t.Fatalf("first = %v", got)
		}
		at = at.Add(fragSlotTTL + time.Second)
		if got := ed.Feed(udpMessage{SessionID: 1, MsgID: 7, FragID: 1, FragCount: 2, Data: []byte("b")}); got != nil {
			t.Fatalf("expired half assembled: %v", got)
		}
		got := ed.Feed(udpMessage{SessionID: 1, MsgID: 7, FragID: 0, FragCount: 2, Data: []byte("c")})
		if got == nil || string(got.Data) != "cb" {
			t.Fatalf("reassembled = %v", got)
		}
	})

	t.Run("unfragmented packet expires a stale slot", func(t *testing.T) {
		at := time.Unix(1_700_000_000, 0)
		ed := &defragger{now: func() time.Time { return at }}
		if got := ed.Feed(udpMessage{SessionID: 1, MsgID: 3, FragID: 0, FragCount: 2, Data: []byte("a")}); got != nil {
			t.Fatalf("first = %v", got)
		}
		at = at.Add(fragSlotTTL + time.Second)
		got := ed.Feed(udpMessage{SessionID: 1, MsgID: 8, FragCount: 1, Data: []byte("plain")})
		if got == nil || string(got.Data) != "plain" {
			t.Fatalf("plain = %v", got)
		}
		if len(ed.slots) != 0 {
			t.Fatalf("slots = %d", len(ed.slots))
		}
	})

	t.Run("slot cap drops the oldest incomplete message", func(t *testing.T) {
		cd := &defragger{}
		for i := 0; i < maxFragSlots; i++ {
			got := cd.Feed(udpMessage{SessionID: uint32(i), MsgID: 1, FragID: 0, FragCount: 2, Data: []byte{byte(i)}})
			if got != nil {
				t.Fatalf("slot %d = %v", i, got)
			}
		}
		if got := cd.Feed(udpMessage{SessionID: uint32(maxFragSlots), MsgID: 1, FragID: 0, FragCount: 2, Data: []byte("n")}); got != nil {
			t.Fatalf("overflow = %v", got)
		}
		if len(cd.slots) > maxFragSlots {
			t.Fatalf("slots = %d", len(cd.slots))
		}
		got := cd.Feed(udpMessage{SessionID: 0, MsgID: 1, FragID: 1, FragCount: 2, Data: []byte("Z")})
		if got != nil {
			t.Fatalf("evicted slot reassembled: %q", got.Data)
		}
	})
}
