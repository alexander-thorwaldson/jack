package msg

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var dmCmd = &cobra.Command{
	Use:   "dm",
	Short: "Direct message commands",
	Long:  "Send and read direct messages by username.",
}

var dmSendCmd = &cobra.Command{
	Use:   "send <user> <message...>",
	Short: "Send a direct message to a user",
	Long:  "Send a direct message by username. Creates a DM room if one doesn't exist.",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		message := strings.Join(args[1:], " ")
		if message == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			message = strings.TrimRight(string(data), "\n")
		}
		return runDMSend(args[0], message, client.WhoAmI, client.GetDirectRooms, client.SetDirectRooms, client.Send, client.CreateDMRoom, client.GetProfile, client.SetRoomAlias, client.ResolveAlias, client.Join)
	},
}

var dmReadCmd = &cobra.Command{
	Use:   "read <user>",
	Short: "Read DM history with a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		return runDMRead(args[0], limit, jsonFlag, client.WhoAmI, client.GetDirectRooms, client.Messages, client.ResolveAlias, client.Join)
	},
}

func init() {
	dmReadCmd.Flags().IntP("limit", "n", 20, "number of messages to retrieve")
	dmReadCmd.Flags().Bool("json", false, "output messages as JSON")
	dmCmd.AddCommand(dmSendCmd)
	dmCmd.AddCommand(dmReadCmd)
	Cmd.AddCommand(dmCmd)
}

type directRoomGetter func(userID string) (map[string][]string, error)
type directRoomSetter func(userID string, rooms map[string][]string) error

func resolveUserID(target string) string {
	if strings.HasPrefix(target, "@") {
		return target
	}
	return "@" + target + ":" + ServerName(Homeserver)
}

func localpart(userID string) string {
	s := strings.TrimPrefix(userID, "@")
	if idx := strings.Index(s, ":"); idx > 0 {
		return s[:idx]
	}
	return s
}

func dmAliasName(userA, userB string) string {
	parts := []string{localpart(userA), localpart(userB)}
	sort.Strings(parts)
	return "dm-" + parts[0] + "-" + parts[1]
}

// resolveDMRoom looks up the DM room for a target user via m.direct account data.
func resolveDMRoom(targetID string, whoami WhoAmIGetter, getDirect directRoomGetter) (string, error) {
	me, err := whoami()
	if err != nil {
		return "", fmt.Errorf("getting identity: %w", err)
	}
	directs, err := getDirect(me.UserID)
	if err != nil {
		return "", fmt.Errorf("getting direct rooms: %w", err)
	}
	if rooms, ok := directs[targetID]; ok && len(rooms) > 0 {
		return rooms[0], nil
	}
	return "", fmt.Errorf("no DM room with %s", targetID)
}

// resolveDMRoomByAlias attempts to find a DM room via the deterministic alias
// pattern and joins it. This handles the case where the other user created
// the DM room and the current user has a pending invite.
func resolveDMRoomByAlias(targetID string, whoami WhoAmIGetter, resolve AliasResolver, join RoomJoiner) (string, error) {
	me, err := whoami()
	if err != nil {
		return "", err
	}
	aliasName := dmAliasName(me.UserID, targetID)
	alias := "#" + aliasName + ":" + ServerName(Homeserver)
	resp, err := resolve(alias)
	if err != nil {
		return "", fmt.Errorf("resolving DM alias %q: %w", alias, err)
	}
	roomID, err := join(resp.RoomID)
	if err != nil {
		return "", fmt.Errorf("joining DM room: %w", err)
	}
	return roomID, nil
}

func runDMSend(target, message string, whoami WhoAmIGetter, getDirect directRoomGetter, setDirect directRoomSetter, send MessageSender, createDM DMRoomCreator, checkProfile ProfileChecker, setAlias RoomAliasCreator, resolve AliasResolver, join RoomJoiner) error {
	targetID := resolveUserID(target)

	me, err := whoami()
	if err != nil {
		return fmt.Errorf("getting identity: %w", err)
	}

	directs, err := getDirect(me.UserID)
	if err != nil {
		return fmt.Errorf("getting direct rooms: %w", err)
	}

	var roomID string
	if rooms, ok := directs[targetID]; ok && len(rooms) > 0 {
		roomID = rooms[0]
	}

	// Try joining an existing DM room via deterministic alias before creating
	// a new one. This handles the case where the other user already created
	// the room and we have a pending invite.
	if roomID == "" {
		if found, resolveErr := resolveDMRoomByAlias(targetID, func() (*WhoAmIResponse, error) { return me, nil }, resolve, join); resolveErr == nil {
			roomID = found
			directs[targetID] = []string{roomID}
			if setErr := setDirect(me.UserID, directs); setErr != nil {
				return fmt.Errorf("updating direct rooms: %w", setErr)
			}
		}
	}

	if roomID == "" {
		if profileErr := checkProfile(targetID); profileErr != nil {
			return fmt.Errorf("user %s does not exist", targetID)
		}
		room, createErr := createDM(targetID)
		if createErr != nil {
			return fmt.Errorf("creating DM room: %w", createErr)
		}
		roomID = room.RoomID

		// Register alias for the DM room.
		alias := "#" + dmAliasName(me.UserID, targetID) + ":" + ServerName(Homeserver)
		_ = setAlias(alias, roomID) // best-effort; alias may already exist

		directs[targetID] = []string{roomID}
		if setErr := setDirect(me.UserID, directs); setErr != nil {
			return fmt.Errorf("updating direct rooms: %w", setErr)
		}
	}

	eventID, err := send(roomID, message)
	if err != nil {
		return err
	}
	fmt.Println(eventID)
	return nil
}

func runDMRead(target string, limit int, jsonOut bool, whoami WhoAmIGetter, getDirect directRoomGetter, read MessageReader, resolve AliasResolver, join RoomJoiner) error {
	targetID := resolveUserID(target)
	roomID, err := resolveDMRoom(targetID, whoami, getDirect)
	if err != nil {
		// Fallback: find the DM room via its deterministic alias and join it.
		// This handles the case where the other user created the room.
		roomID, err = resolveDMRoomByAlias(targetID, whoami, resolve, join)
		if err != nil {
			return fmt.Errorf("no DM room with %s", targetID)
		}
	}
	if jsonOut {
		return runReadJSON(roomID, limit, read)
	}
	return runRead(roomID, limit, read)
}
