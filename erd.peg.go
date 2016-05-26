package main

import (
	"strings"
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleroot
	rulesep
	rulespaces
	ruletable_def
	ruletable_lb
	ruletable_rb
	ruletable_name
	ruletable_description
	rulecolumns
	rulecolumn
	rulecolumn_description
	ruledot
	rulecolumn_name_with_relation
	rulecolumn_name_only
	rulecolumn_name
	rulerarrow
	rulerdotarrow
	rulerlinearrow
	ruletarget_table_name
	ruletarget_column_name
	ruleEOT
	ruleAction0
	rulePegText
	ruleAction1
	ruleAction2
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8
	ruleAction9

	rulePre_
	rule_In_
	rule_Suf
)

var rul3s = [...]string{
	"Unknown",
	"root",
	"sep",
	"spaces",
	"table_def",
	"table_lb",
	"table_rb",
	"table_name",
	"table_description",
	"columns",
	"column",
	"column_description",
	"dot",
	"column_name_with_relation",
	"column_name_only",
	"column_name",
	"rarrow",
	"rdotarrow",
	"rlinearrow",
	"target_table_name",
	"target_column_name",
	"EOT",
	"Action0",
	"PegText",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next uint32, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(string(([]rune(buffer)[node.begin:node.end]))))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next uint32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: uint32(t.begin), end: uint32(t.end), next: uint32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = uint32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, uint32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth uint32, index int) {
	t.tree[index] = token32{pegRule: rule, begin: uint32(begin), end: uint32(end), next: uint32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/*func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2 * len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}*/

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type Parser struct {
	Tables []Table
	table  *Table
	column *Column

	Buffer string
	buffer []rune
	rules  [33]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	Pretty bool
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *Parser
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *Parser) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *Parser) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *Parser) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
			text = string(_buffer[begin:end])

		case ruleAction0:

			p.Tables = append(p.Tables, *p.table)

		case ruleAction1:

			p.table = &Table{
				Name:        text,
				Columns:     make([]Column, 0),
				Description: "",
			}

		case ruleAction2:

			p.table.Description = strings.TrimSpace(text)

		case ruleAction3:

			p.table.Columns = append(p.table.Columns, *p.column)

		case ruleAction4:

			p.column.Description = strings.TrimSpace(text)

		case ruleAction5:

			p.column = &Column{
				Name: text,
			}

		case ruleAction6:

			p.column.Relation = &Relation{
				LineType: DotLine,
			}

		case ruleAction7:

			p.column.Relation = &Relation{
				LineType: NormalLine,
			}

		case ruleAction8:

			p.column.Relation.TableName = text

		case ruleAction9:

			p.column.Relation.ColumnName = text

		}
	}
	_, _, _, _, _ = buffer, _buffer, text, begin, end
}

func (p *Parser) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens32{tree: make([]token32, math.MaxInt16)}
	var max token32
	position, depth, tokenIndex, buffer, _rules := uint32(0), uint32(0), 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin uint32) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position, depth}
		}
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 root <- <((sep* table_def)* sep* EOT)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
			l2:
				{
					position3, tokenIndex3, depth3 := position, tokenIndex, depth
				l4:
					{
						position5, tokenIndex5, depth5 := position, tokenIndex, depth
						if !_rules[rulesep]() {
							goto l5
						}
						goto l4
					l5:
						position, tokenIndex, depth = position5, tokenIndex5, depth5
					}
					if !_rules[ruletable_def]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex, depth = position3, tokenIndex3, depth3
				}
			l6:
				{
					position7, tokenIndex7, depth7 := position, tokenIndex, depth
					if !_rules[rulesep]() {
						goto l7
					}
					goto l6
				l7:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
				}
				if !_rules[ruleEOT]() {
					goto l0
				}
				depth--
				add(ruleroot, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 sep <- <('\n' / '\t' / ' ')> */
		func() bool {
			position8, tokenIndex8, depth8 := position, tokenIndex, depth
			{
				position9 := position
				depth++
				{
					position10, tokenIndex10, depth10 := position, tokenIndex, depth
					if buffer[position] != rune('\n') {
						goto l11
					}
					position++
					goto l10
				l11:
					position, tokenIndex, depth = position10, tokenIndex10, depth10
					if buffer[position] != rune('\t') {
						goto l12
					}
					position++
					goto l10
				l12:
					position, tokenIndex, depth = position10, tokenIndex10, depth10
					if buffer[position] != rune(' ') {
						goto l8
					}
					position++
				}
			l10:
				depth--
				add(rulesep, position9)
			}
			return true
		l8:
			position, tokenIndex, depth = position8, tokenIndex8, depth8
			return false
		},
		/* 2 spaces <- <' '+> */
		func() bool {
			position13, tokenIndex13, depth13 := position, tokenIndex, depth
			{
				position14 := position
				depth++
				if buffer[position] != rune(' ') {
					goto l13
				}
				position++
			l15:
				{
					position16, tokenIndex16, depth16 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l16
					}
					position++
					goto l15
				l16:
					position, tokenIndex, depth = position16, tokenIndex16, depth16
				}
				depth--
				add(rulespaces, position14)
			}
			return true
		l13:
			position, tokenIndex, depth = position13, tokenIndex13, depth13
			return false
		},
		/* 3 table_def <- <(table_name sep (':' spaces* table_description spaces*)? table_lb sep+ columns sep+ table_rb)> */
		func() bool {
			position17, tokenIndex17, depth17 := position, tokenIndex, depth
			{
				position18 := position
				depth++
				if !_rules[ruletable_name]() {
					goto l17
				}
				if !_rules[rulesep]() {
					goto l17
				}
				{
					position19, tokenIndex19, depth19 := position, tokenIndex, depth
					if buffer[position] != rune(':') {
						goto l19
					}
					position++
				l21:
					{
						position22, tokenIndex22, depth22 := position, tokenIndex, depth
						if !_rules[rulespaces]() {
							goto l22
						}
						goto l21
					l22:
						position, tokenIndex, depth = position22, tokenIndex22, depth22
					}
					if !_rules[ruletable_description]() {
						goto l19
					}
				l23:
					{
						position24, tokenIndex24, depth24 := position, tokenIndex, depth
						if !_rules[rulespaces]() {
							goto l24
						}
						goto l23
					l24:
						position, tokenIndex, depth = position24, tokenIndex24, depth24
					}
					goto l20
				l19:
					position, tokenIndex, depth = position19, tokenIndex19, depth19
				}
			l20:
				if !_rules[ruletable_lb]() {
					goto l17
				}
				if !_rules[rulesep]() {
					goto l17
				}
			l25:
				{
					position26, tokenIndex26, depth26 := position, tokenIndex, depth
					if !_rules[rulesep]() {
						goto l26
					}
					goto l25
				l26:
					position, tokenIndex, depth = position26, tokenIndex26, depth26
				}
				if !_rules[rulecolumns]() {
					goto l17
				}
				if !_rules[rulesep]() {
					goto l17
				}
			l27:
				{
					position28, tokenIndex28, depth28 := position, tokenIndex, depth
					if !_rules[rulesep]() {
						goto l28
					}
					goto l27
				l28:
					position, tokenIndex, depth = position28, tokenIndex28, depth28
				}
				if !_rules[ruletable_rb]() {
					goto l17
				}
				depth--
				add(ruletable_def, position18)
			}
			return true
		l17:
			position, tokenIndex, depth = position17, tokenIndex17, depth17
			return false
		},
		/* 4 table_lb <- <'{'> */
		func() bool {
			position29, tokenIndex29, depth29 := position, tokenIndex, depth
			{
				position30 := position
				depth++
				if buffer[position] != rune('{') {
					goto l29
				}
				position++
				depth--
				add(ruletable_lb, position30)
			}
			return true
		l29:
			position, tokenIndex, depth = position29, tokenIndex29, depth29
			return false
		},
		/* 5 table_rb <- <('}' Action0)> */
		func() bool {
			position31, tokenIndex31, depth31 := position, tokenIndex, depth
			{
				position32 := position
				depth++
				if buffer[position] != rune('}') {
					goto l31
				}
				position++
				if !_rules[ruleAction0]() {
					goto l31
				}
				depth--
				add(ruletable_rb, position32)
			}
			return true
		l31:
			position, tokenIndex, depth = position31, tokenIndex31, depth31
			return false
		},
		/* 6 table_name <- <(<([a-z] / [A-Z] / '_')+> Action1)> */
		func() bool {
			position33, tokenIndex33, depth33 := position, tokenIndex, depth
			{
				position34 := position
				depth++
				{
					position35 := position
					depth++
					{
						position38, tokenIndex38, depth38 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l39
						}
						position++
						goto l38
					l39:
						position, tokenIndex, depth = position38, tokenIndex38, depth38
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l40
						}
						position++
						goto l38
					l40:
						position, tokenIndex, depth = position38, tokenIndex38, depth38
						if buffer[position] != rune('_') {
							goto l33
						}
						position++
					}
				l38:
				l36:
					{
						position37, tokenIndex37, depth37 := position, tokenIndex, depth
						{
							position41, tokenIndex41, depth41 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l42
							}
							position++
							goto l41
						l42:
							position, tokenIndex, depth = position41, tokenIndex41, depth41
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l43
							}
							position++
							goto l41
						l43:
							position, tokenIndex, depth = position41, tokenIndex41, depth41
							if buffer[position] != rune('_') {
								goto l37
							}
							position++
						}
					l41:
						goto l36
					l37:
						position, tokenIndex, depth = position37, tokenIndex37, depth37
					}
					depth--
					add(rulePegText, position35)
				}
				if !_rules[ruleAction1]() {
					goto l33
				}
				depth--
				add(ruletable_name, position34)
			}
			return true
		l33:
			position, tokenIndex, depth = position33, tokenIndex33, depth33
			return false
		},
		/* 7 table_description <- <(<(!('\n' / '{') .)+> Action2)> */
		func() bool {
			position44, tokenIndex44, depth44 := position, tokenIndex, depth
			{
				position45 := position
				depth++
				{
					position46 := position
					depth++
					{
						position49, tokenIndex49, depth49 := position, tokenIndex, depth
						{
							position50, tokenIndex50, depth50 := position, tokenIndex, depth
							if buffer[position] != rune('\n') {
								goto l51
							}
							position++
							goto l50
						l51:
							position, tokenIndex, depth = position50, tokenIndex50, depth50
							if buffer[position] != rune('{') {
								goto l49
							}
							position++
						}
					l50:
						goto l44
					l49:
						position, tokenIndex, depth = position49, tokenIndex49, depth49
					}
					if !matchDot() {
						goto l44
					}
				l47:
					{
						position48, tokenIndex48, depth48 := position, tokenIndex, depth
						{
							position52, tokenIndex52, depth52 := position, tokenIndex, depth
							{
								position53, tokenIndex53, depth53 := position, tokenIndex, depth
								if buffer[position] != rune('\n') {
									goto l54
								}
								position++
								goto l53
							l54:
								position, tokenIndex, depth = position53, tokenIndex53, depth53
								if buffer[position] != rune('{') {
									goto l52
								}
								position++
							}
						l53:
							goto l48
						l52:
							position, tokenIndex, depth = position52, tokenIndex52, depth52
						}
						if !matchDot() {
							goto l48
						}
						goto l47
					l48:
						position, tokenIndex, depth = position48, tokenIndex48, depth48
					}
					depth--
					add(rulePegText, position46)
				}
				if !_rules[ruleAction2]() {
					goto l44
				}
				depth--
				add(ruletable_description, position45)
			}
			return true
		l44:
			position, tokenIndex, depth = position44, tokenIndex44, depth44
			return false
		},
		/* 8 columns <- <(column (sep* column)*)> */
		func() bool {
			position55, tokenIndex55, depth55 := position, tokenIndex, depth
			{
				position56 := position
				depth++
				if !_rules[rulecolumn]() {
					goto l55
				}
			l57:
				{
					position58, tokenIndex58, depth58 := position, tokenIndex, depth
				l59:
					{
						position60, tokenIndex60, depth60 := position, tokenIndex, depth
						if !_rules[rulesep]() {
							goto l60
						}
						goto l59
					l60:
						position, tokenIndex, depth = position60, tokenIndex60, depth60
					}
					if !_rules[rulecolumn]() {
						goto l58
					}
					goto l57
				l58:
					position, tokenIndex, depth = position58, tokenIndex58, depth58
				}
				depth--
				add(rulecolumns, position56)
			}
			return true
		l55:
			position, tokenIndex, depth = position55, tokenIndex55, depth55
			return false
		},
		/* 9 column <- <((column_name_with_relation / column_name_only) (spaces* ':' spaces* column_description)? Action3)> */
		func() bool {
			position61, tokenIndex61, depth61 := position, tokenIndex, depth
			{
				position62 := position
				depth++
				{
					position63, tokenIndex63, depth63 := position, tokenIndex, depth
					if !_rules[rulecolumn_name_with_relation]() {
						goto l64
					}
					goto l63
				l64:
					position, tokenIndex, depth = position63, tokenIndex63, depth63
					if !_rules[rulecolumn_name_only]() {
						goto l61
					}
				}
			l63:
				{
					position65, tokenIndex65, depth65 := position, tokenIndex, depth
				l67:
					{
						position68, tokenIndex68, depth68 := position, tokenIndex, depth
						if !_rules[rulespaces]() {
							goto l68
						}
						goto l67
					l68:
						position, tokenIndex, depth = position68, tokenIndex68, depth68
					}
					if buffer[position] != rune(':') {
						goto l65
					}
					position++
				l69:
					{
						position70, tokenIndex70, depth70 := position, tokenIndex, depth
						if !_rules[rulespaces]() {
							goto l70
						}
						goto l69
					l70:
						position, tokenIndex, depth = position70, tokenIndex70, depth70
					}
					if !_rules[rulecolumn_description]() {
						goto l65
					}
					goto l66
				l65:
					position, tokenIndex, depth = position65, tokenIndex65, depth65
				}
			l66:
				if !_rules[ruleAction3]() {
					goto l61
				}
				depth--
				add(rulecolumn, position62)
			}
			return true
		l61:
			position, tokenIndex, depth = position61, tokenIndex61, depth61
			return false
		},
		/* 10 column_description <- <(<(!'\n' .)+> Action4)> */
		func() bool {
			position71, tokenIndex71, depth71 := position, tokenIndex, depth
			{
				position72 := position
				depth++
				{
					position73 := position
					depth++
					{
						position76, tokenIndex76, depth76 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l76
						}
						position++
						goto l71
					l76:
						position, tokenIndex, depth = position76, tokenIndex76, depth76
					}
					if !matchDot() {
						goto l71
					}
				l74:
					{
						position75, tokenIndex75, depth75 := position, tokenIndex, depth
						{
							position77, tokenIndex77, depth77 := position, tokenIndex, depth
							if buffer[position] != rune('\n') {
								goto l77
							}
							position++
							goto l75
						l77:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
						}
						if !matchDot() {
							goto l75
						}
						goto l74
					l75:
						position, tokenIndex, depth = position75, tokenIndex75, depth75
					}
					depth--
					add(rulePegText, position73)
				}
				if !_rules[ruleAction4]() {
					goto l71
				}
				depth--
				add(rulecolumn_description, position72)
			}
			return true
		l71:
			position, tokenIndex, depth = position71, tokenIndex71, depth71
			return false
		},
		/* 11 dot <- <'.'> */
		func() bool {
			position78, tokenIndex78, depth78 := position, tokenIndex, depth
			{
				position79 := position
				depth++
				if buffer[position] != rune('.') {
					goto l78
				}
				position++
				depth--
				add(ruledot, position79)
			}
			return true
		l78:
			position, tokenIndex, depth = position78, tokenIndex78, depth78
			return false
		},
		/* 12 column_name_with_relation <- <(column_name sep rarrow sep target_table_name dot target_column_name)> */
		func() bool {
			position80, tokenIndex80, depth80 := position, tokenIndex, depth
			{
				position81 := position
				depth++
				if !_rules[rulecolumn_name]() {
					goto l80
				}
				if !_rules[rulesep]() {
					goto l80
				}
				if !_rules[rulerarrow]() {
					goto l80
				}
				if !_rules[rulesep]() {
					goto l80
				}
				if !_rules[ruletarget_table_name]() {
					goto l80
				}
				if !_rules[ruledot]() {
					goto l80
				}
				if !_rules[ruletarget_column_name]() {
					goto l80
				}
				depth--
				add(rulecolumn_name_with_relation, position81)
			}
			return true
		l80:
			position, tokenIndex, depth = position80, tokenIndex80, depth80
			return false
		},
		/* 13 column_name_only <- <column_name> */
		func() bool {
			position82, tokenIndex82, depth82 := position, tokenIndex, depth
			{
				position83 := position
				depth++
				if !_rules[rulecolumn_name]() {
					goto l82
				}
				depth--
				add(rulecolumn_name_only, position83)
			}
			return true
		l82:
			position, tokenIndex, depth = position82, tokenIndex82, depth82
			return false
		},
		/* 14 column_name <- <(<([a-z] / [A-Z] / '_')+> Action5)> */
		func() bool {
			position84, tokenIndex84, depth84 := position, tokenIndex, depth
			{
				position85 := position
				depth++
				{
					position86 := position
					depth++
					{
						position89, tokenIndex89, depth89 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l90
						}
						position++
						goto l89
					l90:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l91
						}
						position++
						goto l89
					l91:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
						if buffer[position] != rune('_') {
							goto l84
						}
						position++
					}
				l89:
				l87:
					{
						position88, tokenIndex88, depth88 := position, tokenIndex, depth
						{
							position92, tokenIndex92, depth92 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l93
							}
							position++
							goto l92
						l93:
							position, tokenIndex, depth = position92, tokenIndex92, depth92
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l94
							}
							position++
							goto l92
						l94:
							position, tokenIndex, depth = position92, tokenIndex92, depth92
							if buffer[position] != rune('_') {
								goto l88
							}
							position++
						}
					l92:
						goto l87
					l88:
						position, tokenIndex, depth = position88, tokenIndex88, depth88
					}
					depth--
					add(rulePegText, position86)
				}
				if !_rules[ruleAction5]() {
					goto l84
				}
				depth--
				add(rulecolumn_name, position85)
			}
			return true
		l84:
			position, tokenIndex, depth = position84, tokenIndex84, depth84
			return false
		},
		/* 15 rarrow <- <(rdotarrow / rlinearrow)> */
		func() bool {
			position95, tokenIndex95, depth95 := position, tokenIndex, depth
			{
				position96 := position
				depth++
				{
					position97, tokenIndex97, depth97 := position, tokenIndex, depth
					if !_rules[rulerdotarrow]() {
						goto l98
					}
					goto l97
				l98:
					position, tokenIndex, depth = position97, tokenIndex97, depth97
					if !_rules[rulerlinearrow]() {
						goto l95
					}
				}
			l97:
				depth--
				add(rulerarrow, position96)
			}
			return true
		l95:
			position, tokenIndex, depth = position95, tokenIndex95, depth95
			return false
		},
		/* 16 rdotarrow <- <('.' '.' '>' Action6)> */
		func() bool {
			position99, tokenIndex99, depth99 := position, tokenIndex, depth
			{
				position100 := position
				depth++
				if buffer[position] != rune('.') {
					goto l99
				}
				position++
				if buffer[position] != rune('.') {
					goto l99
				}
				position++
				if buffer[position] != rune('>') {
					goto l99
				}
				position++
				if !_rules[ruleAction6]() {
					goto l99
				}
				depth--
				add(rulerdotarrow, position100)
			}
			return true
		l99:
			position, tokenIndex, depth = position99, tokenIndex99, depth99
			return false
		},
		/* 17 rlinearrow <- <('-' '>' Action7)> */
		func() bool {
			position101, tokenIndex101, depth101 := position, tokenIndex, depth
			{
				position102 := position
				depth++
				if buffer[position] != rune('-') {
					goto l101
				}
				position++
				if buffer[position] != rune('>') {
					goto l101
				}
				position++
				if !_rules[ruleAction7]() {
					goto l101
				}
				depth--
				add(rulerlinearrow, position102)
			}
			return true
		l101:
			position, tokenIndex, depth = position101, tokenIndex101, depth101
			return false
		},
		/* 18 target_table_name <- <(<([a-z] / [A-Z] / '_')+> Action8)> */
		func() bool {
			position103, tokenIndex103, depth103 := position, tokenIndex, depth
			{
				position104 := position
				depth++
				{
					position105 := position
					depth++
					{
						position108, tokenIndex108, depth108 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l109
						}
						position++
						goto l108
					l109:
						position, tokenIndex, depth = position108, tokenIndex108, depth108
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l110
						}
						position++
						goto l108
					l110:
						position, tokenIndex, depth = position108, tokenIndex108, depth108
						if buffer[position] != rune('_') {
							goto l103
						}
						position++
					}
				l108:
				l106:
					{
						position107, tokenIndex107, depth107 := position, tokenIndex, depth
						{
							position111, tokenIndex111, depth111 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l112
							}
							position++
							goto l111
						l112:
							position, tokenIndex, depth = position111, tokenIndex111, depth111
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l113
							}
							position++
							goto l111
						l113:
							position, tokenIndex, depth = position111, tokenIndex111, depth111
							if buffer[position] != rune('_') {
								goto l107
							}
							position++
						}
					l111:
						goto l106
					l107:
						position, tokenIndex, depth = position107, tokenIndex107, depth107
					}
					depth--
					add(rulePegText, position105)
				}
				if !_rules[ruleAction8]() {
					goto l103
				}
				depth--
				add(ruletarget_table_name, position104)
			}
			return true
		l103:
			position, tokenIndex, depth = position103, tokenIndex103, depth103
			return false
		},
		/* 19 target_column_name <- <(<([a-z] / [A-Z] / '_')+> Action9)> */
		func() bool {
			position114, tokenIndex114, depth114 := position, tokenIndex, depth
			{
				position115 := position
				depth++
				{
					position116 := position
					depth++
					{
						position119, tokenIndex119, depth119 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l120
						}
						position++
						goto l119
					l120:
						position, tokenIndex, depth = position119, tokenIndex119, depth119
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l121
						}
						position++
						goto l119
					l121:
						position, tokenIndex, depth = position119, tokenIndex119, depth119
						if buffer[position] != rune('_') {
							goto l114
						}
						position++
					}
				l119:
				l117:
					{
						position118, tokenIndex118, depth118 := position, tokenIndex, depth
						{
							position122, tokenIndex122, depth122 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l123
							}
							position++
							goto l122
						l123:
							position, tokenIndex, depth = position122, tokenIndex122, depth122
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l124
							}
							position++
							goto l122
						l124:
							position, tokenIndex, depth = position122, tokenIndex122, depth122
							if buffer[position] != rune('_') {
								goto l118
							}
							position++
						}
					l122:
						goto l117
					l118:
						position, tokenIndex, depth = position118, tokenIndex118, depth118
					}
					depth--
					add(rulePegText, position116)
				}
				if !_rules[ruleAction9]() {
					goto l114
				}
				depth--
				add(ruletarget_column_name, position115)
			}
			return true
		l114:
			position, tokenIndex, depth = position114, tokenIndex114, depth114
			return false
		},
		/* 20 EOT <- <!.> */
		func() bool {
			position125, tokenIndex125, depth125 := position, tokenIndex, depth
			{
				position126 := position
				depth++
				{
					position127, tokenIndex127, depth127 := position, tokenIndex, depth
					if !matchDot() {
						goto l127
					}
					goto l125
				l127:
					position, tokenIndex, depth = position127, tokenIndex127, depth127
				}
				depth--
				add(ruleEOT, position126)
			}
			return true
		l125:
			position, tokenIndex, depth = position125, tokenIndex125, depth125
			return false
		},
		/* 22 Action0 <- <{
		    p.Tables = append(p.Tables, *p.table)
		}> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		nil,
		/* 24 Action1 <- <{
		    p.table = &Table{
		        Name: text,
		        Columns: make([]Column, 0),
		        Description: "",
			   }
		}> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 25 Action2 <- <{
		    p.table.Description = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 26 Action3 <- <{
		    p.table.Columns = append(p.table.Columns, *p.column)
		}> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 27 Action4 <- <{
		    p.column.Description = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 28 Action5 <- <{
			p.column = &Column{
			  Name: text,
			}
		}> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 29 Action6 <- <{
		    p.column.Relation = &Relation{
		        LineType: DotLine,
		    }
		}> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 30 Action7 <- <{
		    p.column.Relation = &Relation{
		        LineType: NormalLine,
		    }
		}> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 31 Action8 <- <{
		    p.column.Relation.TableName = text
		}> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 32 Action9 <- <{
		    p.column.Relation.ColumnName = text
		}> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
	}
	p.rules = _rules
}
