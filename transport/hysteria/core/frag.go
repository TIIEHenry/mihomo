package core

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
	frags []*udpMessage
	count uint8
}

type defragger struct {
	slots map[fragKey]*fragSlot
}

func (d *defragger) Feed(m udpMessage) *udpMessage {
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
	if slot != nil && len(slot.frags) != int(m.FragCount) {
		return nil
	}
	if slot == nil {
		slot = &fragSlot{frags: make([]*udpMessage, m.FragCount)}
		d.slots[key] = slot
	}
	if int(m.FragID) >= len(slot.frags) {
		return nil
	}
	if slot.frags[m.FragID] == nil {
		slot.frags[m.FragID] = &m
		slot.count++
		if int(slot.count) == len(slot.frags) {
			// all fragments received, assemble
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
	}
	return nil
}
