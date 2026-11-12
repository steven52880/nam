// Package cachetest runs the manual cache-rate test: it sends N sequential requests
// with the same long context through NewAPI's normal /v1 relay, pinned to one
// channel, and computes how much of the input was served from the provider cache.
package cachetest

import _ "embed"

// corpus is a public-domain English text (Pride and Prejudice, Project Gutenberg
// #1342, chapters 1-~8, about 68 KB, roughly 15k tokens). It is long enough to pass
// every provider's minimum cacheable prompt size.
//
//go:embed corpus.txt
var corpus string

// questions are asked in order, one per round (cycled when rounds > len).
var questions = []string{
	"Who is the first character mentioned by name in the text, and what is said about them? Answer in one sentence.",
	"What does Mrs. Bennet most want for her daughters? Answer in one sentence.",
	"Name one thing Mr. Bennet does to tease his wife. Answer in one sentence.",
	"How does the text describe Mr. Bingley when he first appears? Answer in one sentence.",
	"Which of the Bennet sisters is mentioned as the eldest? Answer in one sentence.",
	"What is said about Mr. Darcy's manners at the ball? Answer in one sentence.",
	"What does Charlotte Lucas think about Mr. Darcy's pride? Answer in one sentence.",
	"Summarize the tone of the opening chapters in one sentence.",
}
