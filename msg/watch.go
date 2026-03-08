package msg

// watchMessage is the JSON structure for watch/check output.
type watchMessage struct {
	Type     string `json:"type"`
	RoomID   string `json:"room_id"`
	RoomName string `json:"room_name,omitempty"`
	Sender   string `json:"sender"`
	Body     string `json:"body"`
	EventID  string `json:"event_id,omitempty"`
}

// inviteInfo summarises a pending invite for display.
type inviteInfo struct {
	RoomID string
	Name   string
	Sender string
}

// parseInvites extracts invite info from a sync response's invite map.
func parseInvites(invites map[string]SyncInvitedRoom) []inviteInfo {
	out := make([]inviteInfo, 0, len(invites))
	for roomID, room := range invites {
		inv := inviteInfo{RoomID: roomID}
		for _, ev := range room.InviteState.Events {
			switch ev.Type {
			case "m.room.name":
				if name, ok := ev.Content["name"].(string); ok {
					inv.Name = name
				}
			case "m.room.member":
				if membership, ok := ev.Content["membership"].(string); ok && membership == "invite" {
					inv.Sender = ev.Sender
				}
			}
		}
		out = append(out, inv)
	}
	return out
}

// pollInterval is the maximum duration (in seconds) for a single sync request.
const pollInterval = 5
