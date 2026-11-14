package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	normalRounds = 4
	hardRounds   = 11
	quizTime     = 3 * time.Minute
)

type QuizResult string

const (
	QuizPassed      QuizResult = "passed"
	QuizWrong       QuizResult = "wrong_answer"
	QuizTimedOut    QuizResult = "timed_out"
	QuizError       QuizResult = "error"
)

type QuizMode int

const (
	QuizModeEphemeral QuizMode = iota // edit original interaction response (deferred ephemeral)
	QuizModePublic                     // edit original/public interaction response
)

type QuizSession struct {
	UserID         string
	GuildID        string
	Questions      []Question
	Index          int // 0-based
	Mode           QuizMode
	IncludeMention bool
	// For editing: interaction token/app of the original response context
	AppID       string
	Token       string // interaction token for webhook edits
	Timer       *time.Timer
	Done        chan QuizResult
	Finished    bool
}

type QuizManager struct {
	mu       sync.Mutex
	sessions map[string]*QuizSession // by userID
}

func NewQuizManager() *QuizManager { return &QuizManager{sessions: map[string]*QuizSession{}} }

func (m *QuizManager) Get(userID string) *QuizSession { m.mu.Lock(); defer m.mu.Unlock(); return m.sessions[userID] }
func (m *QuizManager) Set(s *QuizSession)             { m.mu.Lock(); m.sessions[s.UserID] = s; m.mu.Unlock() }
func (m *QuizManager) Delete(userID string) {
	m.mu.Lock()
	if s, ok := m.sessions[userID]; ok {
		if s.Timer != nil { s.Timer.Stop() }
		delete(m.sessions, userID)
	}
	m.mu.Unlock()
}

func isValidQuestion(q Question) bool {
	if q.Text == "" || q.CorrectAnswer == "" || len(q.Options) < 2 { return false }
	for _, o := range q.Options { if o == q.CorrectAnswer { return true } }
	return false
}

func questionContent(q Question, round, total int, mentionPrefix string) string {
	return fmt.Sprintf("%sQuestion %d/%d\n%s", mentionPrefix, round, total, q.Text)
}

func questionComponents(q Question, round int) []discordgo.MessageComponent {
	btns := []discordgo.MessageComponent{}
	for idx, opt := range q.Options {
		label := opt
		if len(label) > 80 { label = label[:80] }
		btns = append(btns, discordgo.Button{
			CustomID: fmt.Sprintf("ans_%d_%d", round, idx),
			Label:    label,
			Style:    discordgo.PrimaryButton,
		})
	}
	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: btns}}
}

func (m *QuizManager) Start(ses *discordgo.Session, i *discordgo.InteractionCreate, state *VerificationState, userID, guildID string, mode QuizMode, includeMention, harder bool) (QuizResult, error) {
	total := normalRounds
	if harder { total = hardRounds }
	qs := CreateQuizQuestions(total, harder)
	if len(qs) == 0 || !isValidQuestion(qs[0]) {
		return QuizError, fmt.Errorf("invalid first question")
	}
	sess := &QuizSession{
		UserID: userID, GuildID: guildID, Questions: qs, Index: 0, Mode: mode, IncludeMention: includeMention,
		AppID: i.AppID, Token: i.Token, Done: make(chan QuizResult, 1),
	}
	state.StartQuiz(userID)
	m.Set(sess)

	mention := ""
	if includeMention { mention = "<@" + userID + "> " }
	content := questionContent(qs[0], 1, len(qs), mention)
	comps := questionComponents(qs[0], 1)

	var err error
	if mode == QuizModeEphemeral || mode == QuizModePublic {
		// edit original interaction response (already created/deferred or replied)
		_, err = ses.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &content, Components: &comps})
	}
	if err != nil {
		m.finish(state, sess, QuizError)
		return QuizError, err
	}
	m.resetTimer(ses, state, sess)
	res := <-sess.Done
	return res, nil
}

func (m *QuizManager) resetTimer(ses *discordgo.Session, state *VerificationState, sess *QuizSession) {
	if sess.Timer != nil { sess.Timer.Stop() }
	sess.Timer = time.AfterFunc(quizTime, func() {
		m.mu.Lock()
		if sess.Finished { m.mu.Unlock(); return }
		sess.Finished = true
		m.mu.Unlock()
		mention := ""
		if sess.IncludeMention { mention = "<@" + sess.UserID + "> " }
		content := mention + "Time expired. Verification failed."
		empty := []discordgo.MessageComponent{}
		_, _ = ses.InteractionResponseEdit(&discordgo.Interaction{AppID: sess.AppID, Token: sess.Token}, &discordgo.WebhookEdit{Content: &content, Components: &empty})
		sess.Done <- QuizTimedOut
		m.cleanup(state, sess.UserID)
	})
}

func (m *QuizManager) finish(state *VerificationState, sess *QuizSession, res QuizResult) {
	m.mu.Lock()
	if sess.Finished { m.mu.Unlock(); return }
	sess.Finished = true
	m.mu.Unlock()
	sess.Done <- res
	m.cleanup(state, sess.UserID)
}

func (m *QuizManager) cleanup(state *VerificationState, userID string) {
	m.Delete(userID)
	state.EndQuiz(userID)
}

func (m *QuizManager) HandleAnswer(ses *discordgo.Session, i *discordgo.InteractionCreate, state *VerificationState) bool {
	user := interactionUser(i)
	if user == nil { return false }
	sess := m.Get(user.ID)
	if sess == nil { return false }
	cid := i.MessageComponentData().CustomID
	parts := strings.Split(cid, "_") // ans_{round}_{idx}
	if len(parts) != 3 || parts[0] != "ans" { return false }
	round, err1 := strconv.Atoi(parts[1])
	idx, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil { return false }
	expectedRound := sess.Index + 1
	if round != expectedRound { 
		// ignore stale
		_ = ses.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
		return true 
	}
	m.mu.Lock()
	if sess.Finished { m.mu.Unlock(); return true }
	m.mu.Unlock()
	q := sess.Questions[sess.Index]
	if idx < 0 || idx >= len(q.Options) || !isValidQuestion(q) {
		content := mentionPrefix(sess) + "An internal error occurred while generating the test."
		empty := []discordgo.MessageComponent{}
		_ = ses.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseUpdateMessage, Data: &discordgo.InteractionResponseData{Content: content, Components: empty}})
		m.finish(state, sess, QuizError)
		return true
	}
	selected := q.Options[idx]
	if selected != q.CorrectAnswer {
		content := mentionPrefix(sess) + "Incorrect answer. Verification failed."
		empty := []discordgo.MessageComponent{}
		_ = ses.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseUpdateMessage, Data: &discordgo.InteractionResponseData{Content: content, Components: empty}})
		m.finish(state, sess, QuizWrong)
		return true
	}
	total := len(sess.Questions)
	if expectedRound == total {
		// passed - defer update on component to ack, then edit original with success? caller may want custom success message.
		// We'll mark passed and let Start/handler decide final message by editing response.
		_ = ses.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
		m.finish(state, sess, QuizPassed)
		return true
	}
	// next question
	sess.Index++
	nq := sess.Questions[sess.Index]
	if !isValidQuestion(nq) {
		content := mentionPrefix(sess) + "An internal error occurred while generating the test."
		empty := []discordgo.MessageComponent{}
		_ = ses.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseUpdateMessage, Data: &discordgo.InteractionResponseData{Content: content, Components: empty}})
		m.finish(state, sess, QuizError)
		return true
	}
	content := questionContent(nq, sess.Index+1, total, mentionPrefix(sess))
	comps := questionComponents(nq, sess.Index+1)
	err := ses.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseUpdateMessage, Data: &discordgo.InteractionResponseData{Content: content, Components: comps}})
	if err != nil {
		log.Printf("failed to update quiz message: %v", err)
		m.finish(state, sess, QuizError)
		return true
	}
	m.resetTimer(ses, state, sess)
	return true
}

func mentionPrefix(sess *QuizSession) string { if sess.IncludeMention { return "<@"+sess.UserID+"> " }; return "" }
