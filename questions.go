package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Question struct {
	Text          string
	CorrectAnswer string
	Options       []string
}

func init() { rand.Seed(time.Now().UnixNano()) }

func rint(min, max int) int {
	if max < min { min, max = max, min }
	return min + rand.Intn(max-min+1)
}
func pickStr(items []string) string { return items[rint(0, len(items)-1)] }
func pickInt(items []int) int       { return items[rint(0, len(items)-1)] }
func shuffleStr(items []string) []string {
	out := append([]string(nil), items...)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func ordinal(n int) string {
	m := n % 100
	if m >= 11 && m <= 13 { return fmt.Sprintf("%dth", n) }
	switch n % 10 {
	case 1: return fmt.Sprintf("%dst", n)
	case 2: return fmt.Sprintf("%dnd", n)
	case 3: return fmt.Sprintf("%drd", n)
	}
	return fmt.Sprintf("%dth", n)
}

func uniqueWrongAnswers(count int, makeCandidate func() string, isInvalid func(string) bool, fallback func(int) string) []string {
	found := []string{}
	seen := map[string]bool{}
	for i := 0; i < 250 && len(found) < count; i++ {
		c := makeCandidate()
		if seen[c] || isInvalid(c) { continue }
		seen[c] = true; found = append(found, c)
	}
	for i := 0; len(found) < count && i < 500; i++ {
		c := fallback(i)
		if seen[c] || isInvalid(c) { continue }
		seen[c] = true; found = append(found, c)
	}
	return found
}

func finalizeQuestion(text, correct string, wrongs []string) Question {
	opts := []string{correct}
	for _, w := range wrongs { if w != correct { opts = append(opts, w) } }
	return Question{Text: text, CorrectAnswer: correct, Options: shuffleStr(opts)}
}

func generateInstructionTrap() Question {
	a, b := rint(10,50), rint(10,50)
	trap := []string{"BLUE","ORANGE","RED","GREEN","PURPLE"}
	target := pickStr(trap)
	var wrongColors []string
	for _, w := range trap { if w != target { wrongColors = append(wrongColors, w) } }
	wrongColors = wrongColors[:2]
	return finalizeQuestion(fmt.Sprintf("What is %d + %d?\nSelect the word \"%s\" below anyway.", a,b,target), target, append([]string{strconv.Itoa(a+b)}, wrongColors...))
}

func generateLogicConditionQuestion() Question {
	targetEven := rand.Intn(2)==0
	num := rint(15,45)
	var valid, invalid []int
	for v:=10; v<=60; v++ {
		ok := (targetEven && v%2==0 && v>num) || (!targetEven && v%2!=0 && v<num)
		if ok { valid = append(valid,v) } else { invalid = append(invalid,v) }
	}
	correct := pickInt(valid)
	rand.Shuffle(len(invalid), func(i,j int){invalid[i],invalid[j]=invalid[j],invalid[i]})
	wr := []string{}
	for i:=0;i<3 && i<len(invalid);i++{wr=append(wr,strconv.Itoa(invalid[i]))}
	cond := "EVEN and GREATER"
	if !targetEven { cond = "ODD and LESS" }
	return finalizeQuestion(fmt.Sprintf("Select the number that is %s than %d.", cond, num), strconv.Itoa(correct), wr)
}

func generateStringPositionQuestion() Question {
	words := []string{"LAUNCHER","DISCORD","MINECRAFT","SUPPORT","TEXTURE"}
	word := pickStr(words)
	pos := rint(2, len(word)-1)
	correct := string(word[pos-1])
	wrongs := uniqueWrongAnswers(3,
		func() string { return string(rune('A'+rint(0,25))) },
		func(c string) bool { return c==correct },
		func(i int) string { return string(rune('A'+(i%26))) },
	)
	return finalizeQuestion(fmt.Sprintf("What is the %s letter in the word \"%s\"?", ordinal(pos), word), correct, wrongs)
}

func generateNegativeLogicQuestion() Question {
	type cat struct{ item string; wrong []string; same []string }
	cats := []cat{
		{"APPLE", []string{"CAR","PLANE","BUS"}, []string{"BANANA","GRAPE","PEACH"}},
		{"DOG", []string{"TABLE","CHAIR","DESK"}, []string{"CAT","WOLF","LION"}},
		{"RED", []string{"SPOON","FORK","KNIFE"}, []string{"BLUE","GREEN","YELLOW"}},
	}
	c := cats[rint(0,len(cats)-1)]
	correct := pickStr(c.wrong)
	return finalizeQuestion(fmt.Sprintf("Which word does NOT belong in the same general category as \"%s\"?", c.item), correct, shuffleStr(c.same)[:3])
}

func generateAlgebraQuestion() Question {
	a,x,b := rint(2,6), rint(1,10), rint(1,10)
	correct := strconv.Itoa(x)
	wrongs := uniqueWrongAnswers(3,
		func() string { d:=rint(1,5); if rand.Intn(2)==0{d=-d}; v:=x+d; if v<0{v=0}; return strconv.Itoa(v) },
		func(c string) bool { return c==correct },
		func(i int) string { return strconv.Itoa(x+i+1) },
	)
	return finalizeQuestion(fmt.Sprintf("Solve for x: %dx + %d = %d", a,b,a*x+b), correct, wrongs)
}

func gcd(a,b int) int { for b!=0 {a,b=b,a%b}; return a }

func generatePercentageQuestion() Question {
	percent := rint(2,10)*5
	base := 100 / gcd(percent,100)
	minK := (20+base-1)/base; maxK := 90/base
	number := rint(minK,maxK)*base
	correct := (percent*number)/100
	cs := strconv.Itoa(correct)
	opts := []int{-10,-5,-2,2,5,10}
	wrongs := uniqueWrongAnswers(3,
		func() string { v:=correct+opts[rint(0,len(opts)-1)]; if v<0{v=0}; return strconv.Itoa(v) },
		func(c string) bool { return c==cs },
		func(i int) string { return strconv.Itoa(correct+i+1) },
	)
	return finalizeQuestion(fmt.Sprintf("What is %d%% of %d?", percent, number), cs, wrongs)
}

func generateArithmeticQuestion() Question {
	ops := []string{"+","-","*"}
	op := pickStr(ops)
	var n1,n2,correct int
	switch op {
	case "+": n1,n2=rint(10,50),rint(10,50); correct=n1+n2
	case "-": n1,n2=rint(20,100),rint(1,19); correct=n1-n2
	default: n1,n2=rint(3,12),rint(3,12); correct=n1*n2
	}
	cs := strconv.Itoa(correct)
	opts := []int{-10,-7,-4,-2,2,4,7,10}
	wrongs := uniqueWrongAnswers(3,
		func() string { v:=correct+opts[rint(0,len(opts)-1)]; if v<0{v=0}; return strconv.Itoa(v) },
		func(c string) bool { return c==cs },
		func(i int) string { return strconv.Itoa(correct+i+1) },
	)
	return finalizeQuestion(fmt.Sprintf("What is %d %s %d?", n1,op,n2), cs, wrongs)
}

func generateRandomQuestion() Question {
	type q struct{ text string; options []string; correct string }
	qs := []q{
		{"Does the launcher support installing mods?", []string{"Yes, with many loaders","No, impossible","Only with a subscription"}, "Yes, with many loaders"},
		{"Is the launcher interface localized?", []string{"Yes, with many languages","No, too costly to translate","Automatically translated by Android"}, "Yes, with many languages"},
		{"What is MJ launcher?", []string{"Successor of MojoLauncher","Separate project worse than Mojo"}, "Successor of MojoLauncher"},
		{"Will you read the rules?", []string{"Yes","No","Maybe in a few days"}, "Yes"},
		{"What is the recommended total device RAM? (not allocation)", []string{"16GB","512MB","4GB","Any amount is enough"}, "4GB"},
		{"Where would you go if you need help?", []string{"The offtopic channels","Dedicated help channels","DM people about it"}, "Dedicated help channels"},
		{"What is the chemical formula of water?", []string{"H2O2","H2O","HO2","2H2O"}, "H2O"},
	}
	s := qs[rint(0,len(qs)-1)]
	return Question{Text:s.text, CorrectAnswer:s.correct, Options:shuffleStr(s.options)}
}

func generateDivisibilityQuestion() Question {
	divs := []int{3,5,7,9,11}; d := pickInt(divs)
	correct := rint(2,90/d)*d
	cs := strconv.Itoa(correct)
	wrongs := uniqueWrongAnswers(3,
		func() string { return strconv.Itoa(correct+rint(-5,5)) },
		func(c string) bool { v,err:=strconv.Atoi(c); return err!=nil||v<=0||v==correct||v%d==0 },
		func(i int) string { return strconv.Itoa(correct+i+1) },
	)
	return finalizeQuestion(fmt.Sprintf("Which number is divisible by %d?", d), cs, wrongs)
}

func generatePatternQuestion() Question {
	type pat struct{ seq []int; next int }
	pats := []pat{{[]int{2,4,6,8},10},{[]int{3,6,12,24},48},{[]int{5,10,20,40},80},{[]int{81,27,9,3},1}}
	p := pats[rint(0,len(pats)-1)]
	cs := strconv.Itoa(p.next)
	inSeq := map[int]bool{}; for _,v:=range p.seq{inSeq[v]=true}
	wrongs := uniqueWrongAnswers(3,
		func() string { return strconv.Itoa(p.next+rint(-5,5)) },
		func(c string) bool { v,err:=strconv.Atoi(c); return err!=nil||v<=0||v==p.next||inSeq[v] },
		func(i int) string { return strconv.Itoa(p.next+i+1) },
	)
	parts := make([]string,len(p.seq)); for i,v:=range p.seq{parts[i]=strconv.Itoa(v)}
	return finalizeQuestion("What comes next? "+strings.Join(parts,", ")+", ___", cs, wrongs)
}

func generateComparisonQuestion() Question {
	vals := uniqueWrongAnswers(4, func() string {
		switch rint(1,3){
		case 1: return strconv.Itoa(rint(1,20))
		case 2:
			s := fmt.Sprintf("%.1f", float64(rint(2,18))/2.0)
			s = strings.TrimSuffix(s, ".0")
			return s
		default: return fmt.Sprintf("%d.%d", rint(1,10), rint(1,9))
		}
	}, func(string) bool { return false }, func(i int) string { return strconv.Itoa(21+i) })
	max := vals[0]; maxf,_ := strconv.ParseFloat(max,64)
	for _,v := range vals[1:] { f,_:=strconv.ParseFloat(v,64); if f>maxf {maxf=f; max=v} }
	var wrongs []string
	for _,v:=range vals { if v!=max {wrongs=append(wrongs,v)} }
	return finalizeQuestion("Which number is the largest?", max, wrongs)
}

type generator func() Question

var allGenerators = []generator{
	generateInstructionTrap, generateStringPositionQuestion, generateLogicConditionQuestion, generateNegativeLogicQuestion,
	generateAlgebraQuestion, generatePercentageQuestion, generateArithmeticQuestion, generateRandomQuestion,
	generateDivisibilityQuestion, generatePatternQuestion, generateComparisonQuestion,
}
var standardGenerators = allGenerators[:4]

func pickGeneratorSequence(gens []generator, count int) []generator {
	pool := append([]generator(nil), gens...)
	rand.Shuffle(len(pool), func(i,j int){pool[i],pool[j]=pool[j],pool[i]})
	for len(pool) < count {
		extra := append([]generator(nil), gens...)
		rand.Shuffle(len(extra), func(i,j int){extra[i],extra[j]=extra[j],extra[i]})
		pool = append(pool, extra...)
	}
	return pool[:count]
}

func CreateQuizQuestions(total int, harder bool) []Question {
	gens := standardGenerators
	if harder { gens = allGenerators }
	seq := pickGeneratorSequence(gens,total)
	out := make([]Question,0,total)
	for _,g := range seq { out = append(out, g()) }
	return out
}

var _ = sort.Ints
