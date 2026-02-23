package channelserver

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
)

// GuildMemberAction is a domain enum for guild member operations.
type GuildMemberAction uint8

const (
	GuildMemberActionAccept GuildMemberAction = iota + 1
	GuildMemberActionReject
	GuildMemberActionKick
)

// ErrUnauthorized is returned when the actor lacks permission for the operation.
var ErrUnauthorized = errors.New("unauthorized")

// ErrUnknownAction is returned for unrecognized guild member actions.
var ErrUnknownAction = errors.New("unknown guild member action")

// OperateMemberResult holds the outcome of a guild member operation.
type OperateMemberResult struct {
	MailRecipientID uint32
	Mail            Mail
}

// GuildService encapsulates guild business logic, sitting between handlers and repos.
type GuildService struct {
	guildRepo GuildRepo
	mailRepo  MailRepo
	charRepo  CharacterRepo
	logger    *zap.Logger
}

// NewGuildService creates a new GuildService.
func NewGuildService(gr GuildRepo, mr MailRepo, cr CharacterRepo, log *zap.Logger) *GuildService {
	return &GuildService{
		guildRepo: gr,
		mailRepo:  mr,
		charRepo:  cr,
		logger:    log,
	}
}

// OperateMember performs a guild member management action (accept/reject/kick).
// The actor must be the guild leader or a sub-leader. On success, a notification
// mail is sent (best-effort) and the result is returned for protocol-level notification.
func (svc *GuildService) OperateMember(actorCharID, targetCharID uint32, action GuildMemberAction) (*OperateMemberResult, error) {
	guild, err := svc.guildRepo.GetByCharID(targetCharID)
	if err != nil || guild == nil {
		return nil, fmt.Errorf("guild lookup for char %d: %w", targetCharID, err)
	}

	actorMember, err := svc.guildRepo.GetCharacterMembership(actorCharID)
	if err != nil || (!actorMember.IsSubLeader() && guild.LeaderCharID != actorCharID) {
		return nil, ErrUnauthorized
	}

	var mail Mail
	switch action {
	case GuildMemberActionAccept:
		err = svc.guildRepo.AcceptApplication(guild.ID, targetCharID)
		mail = Mail{
			RecipientID:     targetCharID,
			Subject:         "Accepted!",
			Body:            fmt.Sprintf("Your application to join 「%s」 was accepted.", guild.Name),
			IsSystemMessage: true,
		}
	case GuildMemberActionReject:
		err = svc.guildRepo.RejectApplication(guild.ID, targetCharID)
		mail = Mail{
			RecipientID:     targetCharID,
			Subject:         "Rejected",
			Body:            fmt.Sprintf("Your application to join 「%s」 was rejected.", guild.Name),
			IsSystemMessage: true,
		}
	case GuildMemberActionKick:
		err = svc.guildRepo.RemoveCharacter(targetCharID)
		mail = Mail{
			RecipientID:     targetCharID,
			Subject:         "Kicked",
			Body:            fmt.Sprintf("You were kicked from 「%s」.", guild.Name),
			IsSystemMessage: true,
		}
	default:
		return nil, ErrUnknownAction
	}

	if err != nil {
		return nil, fmt.Errorf("guild member action %d: %w", action, err)
	}

	// Send mail best-effort
	if mailErr := svc.mailRepo.SendMail(mail.SenderID, mail.RecipientID, mail.Subject, mail.Body, 0, 0, false, true); mailErr != nil {
		svc.logger.Warn("Failed to send guild member operation mail", zap.Error(mailErr))
	}

	return &OperateMemberResult{
		MailRecipientID: targetCharID,
		Mail:            mail,
	}, nil
}
