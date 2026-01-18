// Package internal contains a file to trigger downloads of Google ADK packages for vendoring
package internal

import (
	_ "google.golang.org/adk/agent"
	_ "google.golang.org/adk/agent/llmagent"
	_ "google.golang.org/adk/cmd/launcher"
	_ "google.golang.org/adk/cmd/launcher/full"
	_ "google.golang.org/adk/model/gemini"
	_ "google.golang.org/adk/tool"
	_ "google.golang.org/adk/tool/geminitool"
	_ "google.golang.org/genai"
)
