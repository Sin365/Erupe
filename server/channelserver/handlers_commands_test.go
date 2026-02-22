package channelserver

import (
	"errors"
	"testing"
	"time"

	cfg "erupe-ce/config"
)

// --- mockUserRepoCommands ---

type mockUserRepoCommands struct {
	mockUserRepoGacha

	opResult bool

	// Ban
	bannedUID uint32
	banExpiry *time.Time
	banErr    error
	foundUID  uint32
	foundName string
	findErr   error

	// Timer
	timerState     bool
	timerSetCalled bool
	timerNewState  bool

	// PSN
	psnCount int
	psnSetID string

	// Discord
	discordToken  string
	discordGetErr error
	discordSetTok string

	// Rights
	rightsVal    uint32
	setRightsVal uint32
}

func (m *mockUserRepoCommands) IsOp(_ uint32) (bool, error) { return m.opResult, nil }
func (m *mockUserRepoCommands) GetByIDAndUsername(_ uint32) (uint32, string, error) {
	return m.foundUID, m.foundName, m.findErr
}
func (m *mockUserRepoCommands) BanUser(uid uint32, exp *time.Time) error {
	m.bannedUID = uid
	m.banExpiry = exp
	return m.banErr
}
func (m *mockUserRepoCommands) GetTimer(_ uint32) (bool, error) { return m.timerState, nil }
func (m *mockUserRepoCommands) SetTimer(_ uint32, v bool) error {
	m.timerSetCalled = true
	m.timerNewState = v
	return nil
}
func (m *mockUserRepoCommands) CountByPSNID(_ string) (int, error) { return m.psnCount, nil }
func (m *mockUserRepoCommands) SetPSNID(_ uint32, id string) error {
	m.psnSetID = id
	return nil
}
func (m *mockUserRepoCommands) GetDiscordToken(_ uint32) (string, error) {
	return m.discordToken, m.discordGetErr
}
func (m *mockUserRepoCommands) SetDiscordToken(_ uint32, tok string) error {
	m.discordSetTok = tok
	return nil
}
func (m *mockUserRepoCommands) GetRights(_ uint32) (uint32, error) { return m.rightsVal, nil }
func (m *mockUserRepoCommands) SetRights(_ uint32, v uint32) error {
	m.setRightsVal = v
	return nil
}

// --- helpers ---

func setupCommandsMap(allEnabled bool) {
	commands = map[string]cfg.Command{
		"Ban":      {Name: "Ban", Prefix: "ban", Enabled: allEnabled},
		"Timer":    {Name: "Timer", Prefix: "timer", Enabled: allEnabled},
		"PSN":      {Name: "PSN", Prefix: "psn", Enabled: allEnabled},
		"Reload":   {Name: "Reload", Prefix: "reload", Enabled: allEnabled},
		"KeyQuest": {Name: "KeyQuest", Prefix: "kqf", Enabled: allEnabled},
		"Rights":   {Name: "Rights", Prefix: "rights", Enabled: allEnabled},
		"Course":   {Name: "Course", Prefix: "course", Enabled: allEnabled},
		"Raviente": {Name: "Raviente", Prefix: "ravi", Enabled: allEnabled},
		"Teleport": {Name: "Teleport", Prefix: "tp", Enabled: allEnabled},
		"Discord":  {Name: "Discord", Prefix: "discord", Enabled: allEnabled},
		"Playtime": {Name: "Playtime", Prefix: "playtime", Enabled: allEnabled},
		"Help":     {Name: "Help", Prefix: "help", Enabled: allEnabled},
	}
}

func createCommandSession(repo *mockUserRepoCommands) *Session {
	server := createMockServer()
	server.erupeConfig.CommandPrefix = "!"
	server.userRepo = repo
	server.charRepo = newMockCharacterRepo()
	session := createMockSession(1, server)
	session.userID = 1
	return session
}

func drainChatResponses(s *Session) int {
	count := 0
	for {
		select {
		case <-s.sendPackets:
			count++
		default:
			return count
		}
	}
}

// --- Timer ---

func TestParseChatCommand_Timer_TogglesOn(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{timerState: false}
	s := createCommandSession(repo)

	parseChatCommand(s, "!timer")

	if !repo.timerSetCalled {
		t.Fatal("SetTimer should be called")
	}
	if !repo.timerNewState {
		t.Error("timer should toggle from false to true")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Timer_TogglesOff(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{timerState: true}
	s := createCommandSession(repo)

	parseChatCommand(s, "!timer")

	if !repo.timerSetCalled {
		t.Fatal("SetTimer should be called")
	}
	if repo.timerNewState {
		t.Error("timer should toggle from true to false")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Timer_DisabledNonOp(t *testing.T) {
	setupCommandsMap(false)
	repo := &mockUserRepoCommands{opResult: false}
	s := createCommandSession(repo)

	parseChatCommand(s, "!timer")

	if repo.timerSetCalled {
		t.Error("SetTimer should not be called when disabled for non-op")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (disabled message)", n)
	}
}

func TestParseChatCommand_DisabledCommand_OpCanStillUse(t *testing.T) {
	setupCommandsMap(false)
	repo := &mockUserRepoCommands{opResult: true, timerState: false}
	s := createCommandSession(repo)

	parseChatCommand(s, "!timer")

	if !repo.timerSetCalled {
		t.Error("op should be able to use disabled commands")
	}
}

// --- PSN ---

func TestParseChatCommand_PSN_Success(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{psnCount: 0}
	s := createCommandSession(repo)

	parseChatCommand(s, "!psn MyPSNID")

	if repo.psnSetID != "MyPSNID" {
		t.Errorf("PSN ID = %q, want %q", repo.psnSetID, "MyPSNID")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_PSN_AlreadyExists(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{psnCount: 1}
	s := createCommandSession(repo)

	parseChatCommand(s, "!psn TakenID")

	if repo.psnSetID != "" {
		t.Error("PSN should not be set when ID already exists")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_PSN_MissingArgs(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!psn")

	if repo.psnSetID != "" {
		t.Error("PSN should not be set with missing args")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

// --- Rights ---

func TestParseChatCommand_Rights_Success(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!rights 30")

	if repo.setRightsVal != 30 {
		t.Errorf("rights = %d, want 30", repo.setRightsVal)
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Rights_MissingArgs(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!rights")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

// --- Discord ---

func TestParseChatCommand_Discord_ExistingToken(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{discordToken: "abc-123"}
	s := createCommandSession(repo)

	parseChatCommand(s, "!discord")

	if repo.discordSetTok != "" {
		t.Error("should not generate new token when existing one found")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Discord_NewToken(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{discordGetErr: errors.New("not found")}
	s := createCommandSession(repo)

	parseChatCommand(s, "!discord")

	if repo.discordSetTok == "" {
		t.Error("should generate and set a new token")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

// --- Playtime ---

func TestParseChatCommand_Playtime(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)
	s.playtime = 3661 // 1h 1m 1s
	s.playtimeTime = time.Now()

	parseChatCommand(s, "!playtime")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

// --- Help ---

func TestParseChatCommand_Help_ListsCommands(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!help")

	count := drainChatResponses(s)
	if count != len(commands) {
		t.Errorf("help messages = %d, want %d (one per enabled command)", count, len(commands))
	}
}

// --- Ban ---

func TestParseChatCommand_Ban_Success(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{
		opResult:  true,
		foundUID:  42,
		foundName: "TestUser",
	}
	s := createCommandSession(repo)

	// "211111" converts to CID 1 via ConvertCID (char '2' = value 1)
	parseChatCommand(s, "!ban 211111")

	if repo.bannedUID != 42 {
		t.Errorf("banned UID = %d, want 42", repo.bannedUID)
	}
	if repo.banExpiry != nil {
		t.Error("expiry should be nil for permanent ban")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Ban_WithDuration(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{
		opResult:  true,
		foundUID:  42,
		foundName: "TestUser",
	}
	s := createCommandSession(repo)

	parseChatCommand(s, "!ban 211111 30d")

	if repo.bannedUID != 42 {
		t.Errorf("banned UID = %d, want 42", repo.bannedUID)
	}
	if repo.banExpiry == nil {
		t.Fatal("expiry should not be nil for timed ban")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Ban_NonOp(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{opResult: false}
	s := createCommandSession(repo)

	parseChatCommand(s, "!ban 211111")

	if repo.bannedUID != 0 {
		t.Error("non-op should not be able to ban")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (noOp message)", n)
	}
}

func TestParseChatCommand_Ban_InvalidCID(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{opResult: true}
	s := createCommandSession(repo)

	// "abc" is not 6 chars, ConvertCID returns 0
	parseChatCommand(s, "!ban abc")

	if repo.bannedUID != 0 {
		t.Error("invalid CID should not result in a ban")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Ban_UserNotFound(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{
		opResult: true,
		findErr:  errors.New("not found"),
	}
	s := createCommandSession(repo)

	parseChatCommand(s, "!ban 211111")

	if repo.bannedUID != 0 {
		t.Error("should not ban when user not found")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (noUser message)", n)
	}
}

func TestParseChatCommand_Ban_MissingArgs(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{opResult: true}
	s := createCommandSession(repo)

	parseChatCommand(s, "!ban")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_Ban_DurationUnits(t *testing.T) {
	tests := []struct {
		name     string
		duration string
	}{
		{"seconds", "10s"},
		{"minutes", "5m"},
		{"hours", "2h"},
		{"days", "30d"},
		{"months", "6mo"},
		{"years", "1y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupCommandsMap(true)
			repo := &mockUserRepoCommands{
				opResult:  true,
				foundUID:  1,
				foundName: "User",
			}
			s := createCommandSession(repo)

			parseChatCommand(s, "!ban 211111 "+tt.duration)

			if repo.banExpiry == nil {
				t.Errorf("expiry should not be nil for duration %s", tt.duration)
			}
		})
	}
}

// --- Teleport ---

func TestParseChatCommand_Teleport_Success(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!tp 100 200")

	// Teleport sends a CastedBinary + a chat message = 2 packets
	if n := drainChatResponses(s); n != 2 {
		t.Errorf("packets = %d, want 2 (teleport + message)", n)
	}
}

func TestParseChatCommand_Teleport_MissingArgs(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!tp 100")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

// --- KeyQuest ---

func TestParseChatCommand_KeyQuest_VersionCheck(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)
	s.server.erupeConfig.RealClientMode = cfg.S6 // below G10

	parseChatCommand(s, "!kqf get")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (version error)", n)
	}
}

func TestParseChatCommand_KeyQuest_Get(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)
	s.server.erupeConfig.RealClientMode = cfg.ZZ
	s.kqf = []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	parseChatCommand(s, "!kqf get")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_KeyQuest_Set(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)
	s.server.erupeConfig.RealClientMode = cfg.ZZ

	parseChatCommand(s, "!kqf set 0102030405060708")

	if !s.kqfOverride {
		t.Error("kqfOverride should be true after set")
	}
	if len(s.kqf) != 8 {
		t.Errorf("kqf length = %d, want 8", len(s.kqf))
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

func TestParseChatCommand_KeyQuest_SetInvalid(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)
	s.server.erupeConfig.RealClientMode = cfg.ZZ

	parseChatCommand(s, "!kqf set ABC") // not 16 hex chars

	if s.kqfOverride {
		t.Error("kqfOverride should not be set with invalid hex")
	}
	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (error message)", n)
	}
}

// --- Raviente ---

func TestParseChatCommand_Raviente_NoSemaphore(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!ravi start")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (noPlayers message)", n)
	}
}

func TestParseChatCommand_Raviente_MissingArgs(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!ravi")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (error message)", n)
	}
}

// --- Course ---

func TestParseChatCommand_Course_MissingArgs(t *testing.T) {
	setupCommandsMap(true)
	repo := &mockUserRepoCommands{}
	s := createCommandSession(repo)

	parseChatCommand(s, "!course")

	if n := drainChatResponses(s); n != 1 {
		t.Errorf("chat responses = %d, want 1 (error message)", n)
	}
}

// --- sendServerChatMessage ---

func TestSendServerChatMessage_CommandsContext(t *testing.T) {
	server := createMockServer()
	session := createMockSession(1, server)

	sendServerChatMessage(session, "Hello, World!")

	if n := drainChatResponses(session); n != 1 {
		t.Errorf("chat responses = %d, want 1", n)
	}
}

