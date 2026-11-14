package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const welcomeMessage = "✅ You passed verification! Welcome to the server.\n\n" +
	"**Need help?**\n" +
	"Go to the correct support channel:\n" +
	"• Launcher support → <#1365639455741972563>\n" +
	"• Quick support → <#1521048851526582292>\n" +
	"• FAQ(s) → <#1369295049145188392>/<#1529411767703437352>\n\n" +
	"**Do not ask for help in #offtopic or any non-support channel.**\n" +
	"Help requests posted in the wrong channel will be ignored or removed. Please check the channel name before posting."

const banMessage = "You have been banned from verifying on this server. Contact a staff member if you believe this is a mistake."

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		name := i.ApplicationCommandData().Name
		switch name {
		case "setup-verification":
			b.handleSetupVerification(s, i)
		case "iqtest":
			go b.handleIqTest(s, i)
		case "testiq":
			go b.handleTestIq(s, i)
		case "manage-verification":
			b.handleManageVerification(s, i)
		case "manage-verifyban":
			b.handleManageVerifyBan(s, i)
		default:
			replyEphemeral(s, i, "Unknown command.")
		}
	case discordgo.InteractionMessageComponent:
		cid := i.MessageComponentData().CustomID
		if cid == "start_verification_btn" {
			go b.handleStartVerificationButton(s, i)
			return
		}
		if strings.HasPrefix(cid, "ans_") {
			if b.quiz.HandleAnswer(s, i, b.state) {
				return
			}
		}
	}
}

func replyEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: discordgo.MessageFlagsEphemeral},
	})
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

func editOriginal(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	empty := []discordgo.MessageComponent{}
	_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &content, Components: &empty})
}

func interactionUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member != nil && i.Member.User != nil { return i.Member.User }
	return i.User
}

func formatMinutes(m int) string { if m == 1 { return "1 minute" }; return fmt.Sprintf("%d minutes", m) }

func (b *Bot) handleSetupVerification(s *discordgo.Session, i *discordgo.InteractionCreate) {
	msg := &discordgo.MessageSend{
		Content: "## Server Verification\nClick the button below to answer a few quick questions and gain access to the main channels.",
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "start_verification_btn", Label: "Start Verification", Style: discordgo.SuccessButton},
		}}},
	}
	if _, err := s.ChannelMessageSendComplex(i.ChannelID, msg); err != nil {
		log.Printf("failed to post verification message: %v", err)
		replyEphemeral(s, i, "Failed to post the verification message. Check my permissions in this channel.")
		return
	}
	replyEphemeral(s, i, "Verification message posted.")
}

func getOptUser(data discordgo.ApplicationCommandInteractionData, name string) *discordgo.User {
	for _, o := range data.Options {
		if o.Name == name && o.Type == discordgo.ApplicationCommandOptionUser {
			return o.UserValue(nil)
		}
	}
	return nil
}
func getOptString(data discordgo.ApplicationCommandInteractionData, name string) (string, bool) {
	for _, o := range data.Options {
		if o.Name == name && o.Type == discordgo.ApplicationCommandOptionString {
			return o.StringValue(), true
		}
	}
	return "", false
}

func (b *Bot) fetchMember(s *discordgo.Session, guildID, userID string) (*discordgo.Member, error) {
	return s.GuildMember(guildID, userID)
}

func memberHasRole(m *discordgo.Member, roleID string) bool {
	for _, r := range m.Roles { if r == roleID { return true } }
	return false
}

func hasModerateMembersPerm(s *discordgo.Session, guildID string, m *discordgo.Member) bool {
	if m == nil { return false }
	// best-effort: check Administrator or ModerateMembers on member permissions if available
	if m.Permissions&discordgo.PermissionAdministrator != 0 { return true }
	if m.Permissions&discordgo.PermissionModerateMembers != 0 { return true }
	// fallback: compute from roles
	g, err := s.State.Guild(guildID)
	if err != nil || g == nil { g, err = s.Guild(guildID); if err != nil { return false } }
	perms := int64(0)
	for _, rid := range m.Roles {
		for _, gr := range g.Roles {
			if gr.ID == rid { perms |= gr.Permissions }
		}
	}
	return perms&discordgo.PermissionAdministrator != 0 || perms&discordgo.PermissionModerateMembers != 0
}

func isFailureReason(r QuizResult) bool { return r == QuizWrong || r == QuizTimedOut }

func (b *Bot) handleStartVerificationButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := interactionUser(i)
	if user == nil || i.GuildID == "" { return }
	if b.state.IsInProgress(user.ID) { replyEphemeral(s, i, "You already have a verification test in progress."); return }
	if rem := b.state.GetCooldownRemaining(user.ID); rem > 0 {
		mins := int((rem + time.Minute - 1) / time.Minute)
		replyEphemeral(s, i, fmt.Sprintf("You recently failed verification. Please try again in about %s.", formatMinutes(mins)))
		return
	}
	member, err := b.fetchMember(s, i.GuildID, user.ID)
	if err != nil { replyEphemeral(s, i, "Could not fetch your member information."); return }
	if memberHasRole(member, b.cfg.VerifiedRoleID) { replyEphemeral(s, i, "You are already verified."); return }
	if b.banlist.IsBanned(user.ID) && !hasModerateMembersPerm(s, i.GuildID, member) { replyEphemeral(s, i, banMessage); return }

	// defer ephemeral and run quiz
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral}}); err != nil {
		return
	}
	res, err := b.quiz.Start(s, i, b.state, user.ID, i.GuildID, QuizModeEphemeral, false, false)
	if err != nil { log.Printf("quiz start error: %v", err); editOriginal(s,i,"An error occurred while starting the quiz."); return }
	if res == QuizPassed {
		// recheck ban
		if b.banlist.IsBanned(user.ID) && !hasModerateMembersPerm(s, i.GuildID, member) {
			editOriginal(s, i, banMessage); return
		}
		if err := s.GuildMemberRoleAdd(i.GuildID, user.ID, b.cfg.VerifiedRoleID); err != nil {
			editOriginal(s, i, "You passed, but I could not assign the verified role. Please contact a moderator."); return
		}
		editOriginal(s, i, welcomeMessage)
		return
	}
	// failure: intentionally no cooldown/timeout for self-verification per original
	// final messages for wrong/timeout were already set by quiz handler; leave as-is.
}

func (b *Bot) handleIqTest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	target := getOptUser(data, "target")
	if target == nil { replyEphemeral(s,i,"Invalid target user."); return }
	if target.Bot { replyEphemeral(s,i,"Bots cannot be tested."); return }
	if b.state.IsInProgress(target.ID) { replyEphemeral(s,i,fmt.Sprintf("%s already has a test in progress.", target.Mention())); return }
	tm, err := b.fetchMember(s, i.GuildID, target.ID)
	if err != nil { replyEphemeral(s,i,fmt.Sprintf("%s is not a member of this server.", target.Mention())); return }
	if hasModerateMembersPerm(s, i.GuildID, tm) { replyEphemeral(s,i,fmt.Sprintf("%s has moderator permissions and cannot be tested.", target.Mention())); return }
	if !memberHasRole(tm, b.cfg.VerifiedRoleID) { replyEphemeral(s,i,fmt.Sprintf("%s is not verified.", target.Mention())); return }
	respond(s, i, fmt.Sprintf("%s, please complete the test below.", target.Mention()))
	res, err := b.quiz.Start(s, i, b.state, target.ID, i.GuildID, QuizModePublic, true, true)
	if err != nil { editOriginal(s,i,"An error occurred while running the test."); return }
	if res == QuizPassed {
		editOriginal(s,i, fmt.Sprintf("%s passed the test.", target.Mention())); return
	}
	// failed: remove role, maybe cooldown/timeout
	if err := s.GuildMemberRoleRemove(i.GuildID, target.ID, b.cfg.VerifiedRoleID); err != nil {
		editOriginal(s,i, fmt.Sprintf("%s failed the test, but I could not remove the verified role.", target.Mention())); return
	}
	extra := "."
	if isFailureReason(res) {
		b.state.SetCooldown(target.ID, time.Duration(b.cfg.VerificationTimeoutMins)*time.Minute)
		// best-effort timeout
		until := time.Now().Add(time.Duration(b.cfg.VerificationTimeoutMins) * time.Minute)
		_ = s.GuildMemberTimeout(i.GuildID, target.ID, &until)
		extra = fmt.Sprintf(" and they can try again in %s.", formatMinutes(b.cfg.VerificationTimeoutMins))
	}
	editOriginal(s,i, fmt.Sprintf("%s failed the test. Their verified role has been removed%s", target.Mention(), extra))
}

func (b *Bot) handleTestIq(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	target := getOptUser(data, "target")
	if target == nil { replyEphemeral(s,i,"Invalid target user."); return }
	if target.Bot { replyEphemeral(s,i,"Bots cannot be tested."); return }
	if b.state.IsInProgress(target.ID) { replyEphemeral(s,i,fmt.Sprintf("%s already has a test in progress.", target.Mention())); return }
	if _, err := b.fetchMember(s, i.GuildID, target.ID); err != nil { replyEphemeral(s,i,fmt.Sprintf("%s is not a member of this server.", target.Mention())); return }
	respond(s, i, fmt.Sprintf("%s, please complete the test below.", target.Mention()))
	res, err := b.quiz.Start(s, i, b.state, target.ID, i.GuildID, QuizModePublic, true, true)
	if err != nil { editOriginal(s,i,"An error occurred while running the test."); return }
	if res == QuizPassed { editOriginal(s,i, fmt.Sprintf("%s passed the test.", target.Mention())) } else { editOriginal(s,i, fmt.Sprintf("%s did not pass the test.", target.Mention())) }
}

func (b *Bot) handleManageVerification(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	action, _ := getOptString(data, "action")
	target := getOptUser(data, "target")
	if target == nil { replyEphemeral(s,i,"Invalid target user."); return }
	grant := action == "grant"
	if grant && target.Bot { replyEphemeral(s,i,"Bots cannot be verified."); return }
	tm, err := b.fetchMember(s, i.GuildID, target.ID)
	if err != nil { replyEphemeral(s,i,fmt.Sprintf("%s is not a member of this server.", target.Mention())); return }
	has := memberHasRole(tm, b.cfg.VerifiedRoleID)
	if grant == has {
		if grant { replyEphemeral(s,i,fmt.Sprintf("%s is already verified.", target.Mention())) } else { replyEphemeral(s,i,fmt.Sprintf("%s is not verified.", target.Mention())) }
		return
	}
	if grant { err = s.GuildMemberRoleAdd(i.GuildID, target.ID, b.cfg.VerifiedRoleID) } else { err = s.GuildMemberRoleRemove(i.GuildID, target.ID, b.cfg.VerifiedRoleID) }
	if err != nil {
		verb := "remove"
		if grant { verb = "add" }
		replyEphemeral(s,i,fmt.Sprintf("Could not %s the verified role for %s.", verb, target.Mention()))
		return
	}
	if grant { respond(s,i,fmt.Sprintf("Successfully verified %s.", target.Mention())) } else { respond(s,i,fmt.Sprintf("Removed the verified role from %s.", target.Mention())) }
}

func (b *Bot) handleManageVerifyBan(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	action, _ := getOptString(data, "action")
	target := getOptUser(data, "target")
	reason, _ := getOptString(data, "reason")
	if target == nil { replyEphemeral(s,i,"Invalid target user."); return }
	mod := interactionUser(i)
	if action == "ban" {
		if target.Bot { replyEphemeral(s,i,"Bots cannot be banned from verification."); return }
		tm, _ := b.fetchMember(s, i.GuildID, target.ID)
		if hasModerateMembersPerm(s, i.GuildID, tm) { replyEphemeral(s,i,fmt.Sprintf("%s has moderator permissions and cannot be banned from verification.", target.Mention())); return }
		added, err := b.banlist.Ban(target.ID)
		if err != nil { log.Printf("ban error: %v", err); replyEphemeral(s,i,fmt.Sprintf("Could not ban %s from verification due to an internal error.", target.Mention())); return }
		b.state.ClearCooldown(target.ID)
		if added { b.sendVerificationLog(s, i.GuildID, "Verification Ban", target, mod, reason) }
		if added { respond(s,i,fmt.Sprintf("%s has been banned from verifying.", target.Mention())) } else { respond(s,i,fmt.Sprintf("%s is already banned from verifying.", target.Mention())) }
		return
	}
	removed, err := b.banlist.Unban(target.ID)
	if err != nil { log.Printf("unban error: %v", err); replyEphemeral(s,i,fmt.Sprintf("Could not unban %s due to an internal error.", target.Mention())); return }
	if removed { b.sendVerificationLog(s, i.GuildID, "Verification Unban", target, mod, reason) }
	if removed { respond(s,i,fmt.Sprintf("%s can attempt verification again.", target.Mention())) } else { respond(s,i,fmt.Sprintf("%s is not on the verification banlist.", target.Mention())) }
}

func (b *Bot) sendVerificationLog(s *discordgo.Session, guildID, action string, target, mod *discordgo.User, reason string) {
	if b.cfg.VerdictChannelID == "" || mod == nil { return }
	r := reason
	if strings.TrimSpace(r) == "" { r = "No reason provided" }
	embed := &discordgo.MessageEmbed{
		Title: action,
		Color: 0x9B59B6,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "User", Value: fmt.Sprintf("%s (%s)", target.String(), target.ID), Inline: false},
			{Name: "Moderator", Value: fmt.Sprintf("%s (%s)", mod.String(), mod.ID), Inline: false},
			{Name: "Reason", Value: r, Inline: false},
			{Name: "Time", Value: fmt.Sprintf("<t:%d:F>", time.Now().Unix()), Inline: false},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
	_, _ = s.ChannelMessageSendEmbed(b.cfg.VerdictChannelID, embed)
}
