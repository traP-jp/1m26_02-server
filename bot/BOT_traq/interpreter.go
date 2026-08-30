package main

import (
	"errors"
	"fmt"
	"strings"
)

const botMention = "@BOT_traq"

var (
	errInvalidArgumentCount = errors.New("exactly two arguments are required")
	errNoInterpretation     = errors.New("no valid interpretation after knight moves")
)

type wordKind int

const (
	commandWord wordKind = iota
	targetWord
)

type boardSquare struct {
	word string
	kind wordKind
}

type boardPosition struct {
	row int
	col int
}

var commandBoard = [4][4]*boardSquare{
	{nil, {word: "count", kind: commandWord}, {word: "list", kind: commandWord}, {word: "debug", kind: commandWord}},
	{{word: "open", kind: commandWord}, {word: "send", kind: commandWord}, {word: "reset", kind: commandWord}, {word: "delete", kind: commandWord}},
	{{word: "BOT", kind: targetWord}, {word: "user", kind: targetWord}, {word: "stamp", kind: targetWord}, {word: "channel", kind: targetWord}},
	{nil, {word: "image", kind: targetWord}, {word: "message", kind: targetWord}, {word: "file", kind: targetWord}},
}

var knightMoves = [...]boardPosition{
	{row: -2, col: -1},
	{row: -2, col: 1},
	{row: -1, col: -2},
	{row: -1, col: 2},
	{row: 1, col: -2},
	{row: 1, col: 2},
	{row: 2, col: -1},
	{row: 2, col: 1},
}

func interpretMessage(plainText string) ([]string, error) {
	arguments := make([]string, 0, 2)
	for _, field := range strings.Fields(plainText) {
		if strings.EqualFold(field, botMention) {
			continue
		}
		arguments = append(arguments, field)
	}
	if len(arguments) != 2 {
		return nil, errInvalidArgumentCount
	}
	return interpret(arguments[0], arguments[1])
}

func interpret(firstWord, secondWord string) ([]string, error) {
	commands, err := knightDestinations(firstWord, commandWord)
	if err != nil {
		return nil, err
	}
	targets, err := knightDestinations(secondWord, targetWord)
	if err != nil {
		return nil, err
	}
	if len(commands) == 0 || len(targets) == 0 {
		return nil, errNoInterpretation
	}

	interpretations := make([]string, 0, len(commands)*len(targets))
	for _, command := range commands {
		for _, target := range targets {
			interpretations = append(interpretations, command+" "+target)
		}
	}
	return interpretations, nil
}

func knightDestinations(word string, wantKind wordKind) ([]string, error) {
	position, ok := findWord(word)
	if !ok {
		return nil, fmt.Errorf("unknown word %q", word)
	}

	destinations := make([]string, 0, len(knightMoves))
	for _, move := range knightMoves {
		row := position.row + move.row
		col := position.col + move.col
		if row < 0 || row >= len(commandBoard) || col < 0 || col >= len(commandBoard[row]) {
			continue
		}
		square := commandBoard[row][col]
		if square == nil || square.kind != wantKind {
			continue
		}
		destinations = append(destinations, square.word)
	}
	return destinations, nil
}

func findWord(word string) (boardPosition, bool) {
	for row := range commandBoard {
		for col, square := range commandBoard[row] {
			if square != nil && square.word == word {
				return boardPosition{row: row, col: col}, true
			}
		}
	}
	return boardPosition{}, false
}
