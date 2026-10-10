package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"strconv"
	"strings"
)

var quickPhrases = [...]string{"准备好了", "等我一下", "轮到你了", "好牌！", "这局很精彩", "再来一局", "谢谢", "我先离开啦", "祝你好运"}

func (m *Model) quickChatKey(key string) tea.Cmd {
	if key == "esc" || key == "f3" {
		m.quickChat = false
		return nil
	}
	if key == "delete" {
		m.quickChat = false
		if m.page == pokemonScreen {
			return m.pokemonKey(tea.KeyPressMsg{Code: tea.KeyDelete})
		}
		if m.page == snakeScreen {
			return m.snakeKey("delete")
		}
		return m.leave()
	}
	n, err := strconv.Atoi(key)
	if err != nil || n < 1 || n > len(quickPhrases) || !m.socialReady {
		return nil
	}
	text := quickPhrases[n-1]
	m.quickChat = false
	m.chatOn = false
	m.chat.Blur()
	return m.sendChatText(text)
}
func (m *Model) quickChatView() string {
	rows := []string{titleStyle.Render("聊天短语 · 只需数字"), ""}
	for i, p := range quickPhrases {
		rows = append(rows, strconv.Itoa(i+1)+". "+p)
	}
	rows = append(rows, "", muted.Render("1–9 发送 · F3 / Esc 返回"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, strings.Join(rows, "\n"))
}
