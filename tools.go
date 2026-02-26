//go:build tools

// Package main contains build-time tool imports to keep dependencies in go.mod
// until they are imported by their respective packages in later tasks.
package main

import (
	_ "github.com/charmbracelet/bubbles"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/lipgloss"
	_ "github.com/gorilla/websocket"
	_ "go.uber.org/zap"
)
