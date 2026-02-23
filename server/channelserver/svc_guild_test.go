package channelserver

import (
	"errors"
	"testing"

	"go.uber.org/zap"
)

func newTestGuildService(gr GuildRepo, mr MailRepo) *GuildService {
	logger, _ := zap.NewDevelopment()
	return NewGuildService(gr, mr, nil, logger)
}

func TestGuildService_OperateMember(t *testing.T) {
	tests := []struct {
		name          string
		actorCharID   uint32
		targetCharID  uint32
		action        GuildMemberAction
		guild         *Guild
		membership    *GuildMember
		acceptErr     error
		rejectErr     error
		removeErr     error
		sendErr       error
		wantErr       bool
		wantErrIs     error
		wantAccepted  uint32
		wantRejected  uint32
		wantRemoved   uint32
		wantMailCount int
		wantRecipient uint32
		wantMailSubj  string
	}{
		{
			name:          "accept application as leader",
			actorCharID:   1,
			targetCharID:  42,
			action:        GuildMemberActionAccept,
			guild:         &Guild{ID: 10, Name: "TestGuild", GuildLeader: GuildLeader{LeaderCharID: 1}},
			membership:    &GuildMember{GuildID: 10, CharID: 1, IsLeader: true, OrderIndex: 1},
			wantAccepted:  42,
			wantMailCount: 1,
			wantRecipient: 42,
			wantMailSubj:  "Accepted!",
		},
		{
			name:          "reject application as sub-leader",
			actorCharID:   2,
			targetCharID:  42,
			action:        GuildMemberActionReject,
			guild:         &Guild{ID: 10, Name: "TestGuild", GuildLeader: GuildLeader{LeaderCharID: 1}},
			membership:    &GuildMember{GuildID: 10, CharID: 2, OrderIndex: 2}, // sub-leader
			wantRejected:  42,
			wantMailCount: 1,
			wantRecipient: 42,
			wantMailSubj:  "Rejected",
		},
		{
			name:          "kick member as leader",
			actorCharID:   1,
			targetCharID:  42,
			action:        GuildMemberActionKick,
			guild:         &Guild{ID: 10, Name: "TestGuild", GuildLeader: GuildLeader{LeaderCharID: 1}},
			membership:    &GuildMember{GuildID: 10, CharID: 1, IsLeader: true, OrderIndex: 1},
			wantRemoved:   42,
			wantMailCount: 1,
			wantRecipient: 42,
			wantMailSubj:  "Kicked",
		},
		{
			name:         "unauthorized - not leader or sub",
			actorCharID:  5,
			targetCharID: 42,
			action:       GuildMemberActionAccept,
			guild:        &Guild{ID: 10, Name: "TestGuild", GuildLeader: GuildLeader{LeaderCharID: 1}},
			membership:   &GuildMember{GuildID: 10, CharID: 5, OrderIndex: 10},
			wantErr:      true,
			wantErrIs:    ErrUnauthorized,
		},
		{
			name:         "repo error on accept",
			actorCharID:  1,
			targetCharID: 42,
			action:       GuildMemberActionAccept,
			guild:        &Guild{ID: 10, Name: "TestGuild", GuildLeader: GuildLeader{LeaderCharID: 1}},
			membership:   &GuildMember{GuildID: 10, CharID: 1, IsLeader: true, OrderIndex: 1},
			acceptErr:    errors.New("db error"),
			wantErr:      true,
		},
		{
			name:          "mail error is best-effort",
			actorCharID:   1,
			targetCharID:  42,
			action:        GuildMemberActionAccept,
			guild:         &Guild{ID: 10, Name: "TestGuild", GuildLeader: GuildLeader{LeaderCharID: 1}},
			membership:    &GuildMember{GuildID: 10, CharID: 1, IsLeader: true, OrderIndex: 1},
			sendErr:       errors.New("mail failed"),
			wantAccepted:  42,
			wantMailCount: 1,
			wantRecipient: 42,
			wantMailSubj:  "Accepted!",
		},
		{
			name:         "unknown action",
			actorCharID:  1,
			targetCharID: 42,
			action:       GuildMemberAction(99),
			guild:        &Guild{ID: 10, Name: "TestGuild", GuildLeader: GuildLeader{LeaderCharID: 1}},
			membership:   &GuildMember{GuildID: 10, CharID: 1, IsLeader: true, OrderIndex: 1},
			wantErr:      true,
			wantErrIs:    ErrUnknownAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guildMock := &mockGuildRepoOps{
				membership: tt.membership,
				acceptErr:  tt.acceptErr,
				rejectErr:  tt.rejectErr,
				removeErr:  tt.removeErr,
			}
			guildMock.guild = tt.guild
			mailMock := &mockMailRepo{sendErr: tt.sendErr}

			svc := newTestGuildService(guildMock, mailMock)

			result, err := svc.OperateMember(tt.actorCharID, tt.targetCharID, tt.action)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("Expected error %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.wantAccepted != 0 && guildMock.acceptedCharID != tt.wantAccepted {
				t.Errorf("acceptedCharID = %d, want %d", guildMock.acceptedCharID, tt.wantAccepted)
			}
			if tt.wantRejected != 0 && guildMock.rejectedCharID != tt.wantRejected {
				t.Errorf("rejectedCharID = %d, want %d", guildMock.rejectedCharID, tt.wantRejected)
			}
			if tt.wantRemoved != 0 && guildMock.removedCharID != tt.wantRemoved {
				t.Errorf("removedCharID = %d, want %d", guildMock.removedCharID, tt.wantRemoved)
			}
			if len(mailMock.sentMails) != tt.wantMailCount {
				t.Fatalf("sentMails count = %d, want %d", len(mailMock.sentMails), tt.wantMailCount)
			}
			if tt.wantMailCount > 0 {
				if mailMock.sentMails[0].recipientID != tt.wantRecipient {
					t.Errorf("mail recipientID = %d, want %d", mailMock.sentMails[0].recipientID, tt.wantRecipient)
				}
				if mailMock.sentMails[0].subject != tt.wantMailSubj {
					t.Errorf("mail subject = %q, want %q", mailMock.sentMails[0].subject, tt.wantMailSubj)
				}
			}
			if result.MailRecipientID != tt.targetCharID {
				t.Errorf("result.MailRecipientID = %d, want %d", result.MailRecipientID, tt.targetCharID)
			}
		})
	}
}
