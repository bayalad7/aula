package main

import (
	"regexp"
	"strings"
)

// Languages offered in the code panel (Monaco ids, also valid protocol lang).
var languages = []string{
	"python", "java", "c", "cpp", "csharp", "javascript", "typescript", "go",
	"sql", "html", "css", "shell", "json", "rust", "kotlin", "plaintext",
}

var langHints = []struct {
	lang string
	re   *regexp.Regexp
}{
	{"html", regexp.MustCompile(`(?i)^\s*(<!doctype html|<html|<div|<body|<head)`)},
	{"go", regexp.MustCompile(`(?m)^\s*(package \w+|func \w*\(|import \(\s*$)`)},
	{"cpp", regexp.MustCompile(`(?m)(#include\s*<(iostream|vector|string)>|std::|cout\s*<<)`)},
	{"c", regexp.MustCompile(`(?m)(#include\s*<stdio\.h>|printf\s*\()`)},
	{"java", regexp.MustCompile(`(?m)(public\s+(static\s+)?(class|void)|System\.out\.print)`)},
	{"csharp", regexp.MustCompile(`(?m)(using System;|Console\.Write(Line)?\(|namespace \w+)`)},
	{"python", regexp.MustCompile(`(?m)(^\s*def \w+\(.*\):\s*$|^\s*import \w+\s*$|^\s*from \w+ import |print\()`)},
	{"sql", regexp.MustCompile(`(?im)^\s*(select\s.+\sfrom\s|insert\s+into\s|create\s+table\s|update\s+\w+\s+set\s)`)},
	{"javascript", regexp.MustCompile(`(?m)(console\.log\(|function \w+\(|=>\s*\{|\bconst \w+ =|\blet \w+ =)`)},
	{"css", regexp.MustCompile(`(?m)^[.#]?[\w-]+\s*\{\s*$`)},
	{"shell", regexp.MustCompile(`(?m)^#!/bin/(ba)?sh`)},
}

// guessLang proposes a language for captured code. It only suggests: the
// teacher can change it before emitting. Without a clear hint it keeps prev.
func guessLang(code, prev string) string {
	for _, h := range langHints {
		if h.re.MatchString(code) {
			return h.lang
		}
	}
	if prev != "" {
		return prev
	}
	return "plaintext"
}

// normalizeNewlines turns CRLF and CR into LF, as the protocol carries code.
func normalizeNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
