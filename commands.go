package main

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func registerCommands(s *discordgo.Session, appID string) error {
	permsMod := int64(discordgo.PermissionModerateMembers)
	permsAdmin := int64(discordgo.PermissionAdministrator)
	dmFalse := false
	cmds := []*discordgo.ApplicationCommand{
		{Name: "setup-verification", Description: "Post the verification button in the current channel.", DefaultMemberPermissions: &permsAdmin, DMPermission: &dmFalse},
		{Name: "iqtest", Description: "Force a verified member to retake the verification test; removes the role on failure.", DefaultMemberPermissions: &permsMod, DMPermission: &dmFalse,
			Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "target", Description: "The member to test", Required: true}}},
		{Name: "testiq", Description: "Run the quiz for a member without changing their verification status.", DefaultMemberPermissions: &permsMod, DMPermission: &dmFalse,
			Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "target", Description: "The member to test", Required: true}}},
		{Name: "manage-verification", Description: "Manually grant or revoke the verified role for a member.", DefaultMemberPermissions: &permsMod, DMPermission: &dmFalse,
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Whether to grant or revoke verification", Required: true, Choices: []*discordgo.ApplicationCommandOptionChoice{{Name: "Grant", Value: "grant"}, {Name: "Revoke", Value: "revoke"}}},
				{Type: discordgo.ApplicationCommandOptionUser, Name: "target", Description: "The member to verify or unverify", Required: true},
			}},
		{Name: "manage-verifyban", Description: "Ban or unban a user from completing verification.", DefaultMemberPermissions: &permsMod, DMPermission: &dmFalse,
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Whether to ban or unban the user", Required: true, Choices: []*discordgo.ApplicationCommandOptionChoice{{Name: "Ban", Value: "ban"}, {Name: "Unban", Value: "unban"}}},
				{Type: discordgo.ApplicationCommandOptionUser, Name: "target", Description: "The user to ban or unban from verification", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for the action (logged only)", Required: false},
			}},
	}
	_, err := s.ApplicationCommandBulkOverwrite(appID, "", cmds)
	if err != nil {
		return err
	}
	log.Printf("Registered %d application command(s).", len(cmds))
	return nil
}
