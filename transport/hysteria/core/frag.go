package core

import (
	"bytes"
	"time"
)

const (
	// maxFragSlots bounds incomplete messages retained for one QUIC session.
	maxFragSlots = 64
	// fragSlotTTL drops a message that never receives its remaining fragments.
	fragSlotTTL = 10 * time.Second
)

func fragUDPMessage(m udpMessage, maxSize int) []udpMessage {
	if m.Size() <= maxSize {
		return []udpMessage{m}
	}
	fullPayload := m.Data
	maxPayloadSize := maxSize - m.HeaderSize()
	off := 0
	fragID := uint8(0)
	fragCount := uint8((len(fullPayload) + maxPayloadSize - 1) / maxPayloadSize) // round up
	var frags []udpMessage
	for off < len(fullPayload) {
		payloadSize := len(fullPayload) - off
		if payloadSize > maxPayloadSize {
			payloadSize = maxPayloadSize
		}
		frag := m
		frag.FragID = fragID
		frag.FragCount = fragCount
		frag.Data = fullPayload[off : off+payloadSize]
		frags = append(frags, frag)
		off += payloadSize
		fragID++
	}
	return frags
}

type fragKey struct {
	SessionID uint32
	MsgID     uint16
}

type fragSlot struct {
	frags   []*udpMessage
	count   uint8
	created time.Time
	seq     uint64
}

type defragger struct {
	slots map[fragKey]*fragSlot
	next  uint64
	now   func() time.Time
}

func (d *defragger) clock() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

func (d *defragger) dropExpired(now time.Time) {
	for key, slot := range d.slots {
		if now.Sub(slot.created) > fragSlotTTL {
			delete(d.slots, key)
		}
	}
}

func (d *defragger) evictOldest() {
	var oldest fragKey
	var seq uint64
	found := false
	for key, slot := range d.slots {
		if !found || slot.seq < seq {
			oldest = key
			seq = slot.seq
			found = true
		}
	}
	if found {
		delete(d.slots, oldest)
	}
}

func (d *defragger) Feed(m udpMessage) *udpMessage {
	// Most datagrams are not fragmented. Expire stale slots on those too,
	// or a lost fragment stays until the next fragmented packet.
	now := d.clock()
	if len(d.slots) > 0 {
		d.dropExpired(now)
	}
	if m.FragCount <= 1 {
		return &m
	}
	if m.FragID >= m.FragCount {
		// wtf is this?
		return nil
	}
	if d.slots == nil {
		d.slots = make(map[fragKey]*fragSlot)
	}
	key := fragKey{SessionID: m.SessionID, MsgID: m.MsgID}
	slot := d.slots[key]
	// A count that disagrees belongs to another message that reused this MsgID.
	if slot != nil && len(slot.frags) != int(m.FragCount) {
		delete(d.slots, key)
		slot = nil
	}
	// The same fragment index with a different payload is that reuse as well.
	if slot != nil && slot.frags[m.FragID] != nil && !bytes.Equal(slot.frags[m.FragID].Data, m.Data) {
		delete(d.slots, key)
		slot = nil
	}
	if slot == nil {
		for len(d.slots) >= maxFragSlots {
			d.evictOldest()
		}
		slot = &fragSlot{
			frags:   make([]*udpMessage, m.FragCount),
			created: now,
			seq:     d.next,
		}
		d.next++
		d.slots[key] = slot
	}
	if int(m.FragID) >= len(slot.frags) {
		return nil
	}
	if slot.frags[m.FragID] != nil {
		return nil
	}
	slot.frags[m.FragID] = &m
	slot.count++
	if int(slot.count) != len(slot.frags) {
		return nil
	}
	var data []byte
	for _, frag := range slot.frags {
		data = append(data, frag.Data...)
	}
	delete(d.slots, key)
	m.Data = data
	m.FragID = 0
	m.FragCount = 1
	return &m
}
