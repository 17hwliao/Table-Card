package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/pokemon"
	"github.com/17hwliao/table-card-independent/internal/table"
)

type pokemonProfile struct {
	State      pokemon.State `json:"state"`
	Revision   uint64        `json:"revision"`
	Escrow     bool          `json:"escrow,omitempty"`
	Resolution string        `json:"resolution,omitempty"`
}
type pokemonOffer struct {
	ID       string             `json:"id"`
	Kind     string             `json:"kind"`
	From     string             `json:"from"`
	To       string             `json:"to"`
	FromName string             `json:"fromName"`
	ToName   string             `json:"toName"`
	Slots    [2]int             `json:"slots"`
	Monsters [2]pokemon.Monster `json:"monsters"`
	Expires  int64              `json:"expires"`
}
type pokemonMatch struct {
	IDs     [2]string
	Duel    *pokemon.Duel
	Updated time.Time
}
type pokemonSocialStore struct {
	mu       sync.Mutex
	path     string
	profiles map[string]pokemonProfile
	offers   map[string]pokemonOffer
	matches  map[string]*pokemonMatch
	ready    map[string]bool // Current synchronized online-center sessions only.
	loadErr  error
	recovery map[string]pokemonProfile
}

func newPokemonSocialStore() *pokemonSocialStore {
	dir := os.Getenv("TABLE_CARD_DATA_DIR")
	if dir == "" {
		dir, _ = os.UserConfigDir()
		dir = filepath.Join(dir, "Table-Card", "server")
	}
	x := &pokemonSocialStore{path: filepath.Join(dir, "pokemon-social.json"), profiles: map[string]pokemonProfile{}, offers: map[string]pokemonOffer{}, matches: map[string]*pokemonMatch{}, ready: map[string]bool{}}
	f, err := os.Open(x.path)
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(f, (64<<20)+1))
		_ = f.Close()
	}
	if err == nil {
		if len(data) > 64<<20 {
			err = errors.New("联机档案文件超过64MB")
		} else {
			err = json.Unmarshal(data, &x.profiles)
		}
		if err == nil && x.profiles == nil {
			err = errors.New("联机档案必须是有效对象")
		}
		if err == nil {
			for id, p := range x.profiles {
				identity, e := hex.DecodeString(id)
				if e != nil || len(identity) != 16 || !table.ValidVisibleText(p.State.Name, 32) {
					err = errors.New("联机档案身份或昵称无效")
					break
				}
				if e := pokemon.NormalizeData(&p.State); e != nil {
					err = e
					break
				}
				x.profiles[id] = p
				if p.Resolution != "" && (!p.Escrow || (p.Resolution != "win" && p.Resolution != "loss" && p.Resolution != "refund")) {
					err = errors.New("押金结果记录无效")
					break
				}
			}
		}
	} else if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		x.loadErr = err
		x.profiles = map[string]pokemonProfile{}
		return x
	}
	// Recover saved results; only unfinished duels refund both reserved stakes.
	recovered := false
	next := x.copyProfiles()
	for id, p := range next {
		if p.Escrow {
			if p.Resolution == "win" {
				p.State.Money += 600
			} else if p.Resolution != "loss" {
				p.State.Money += 300
			}
			p.Escrow = false
			p.Resolution = ""
			p.Revision++
			p.State.MultiplayerRevision = p.Revision
			if p.State.Money > 1000000000 {
				x.loadErr = errors.New("押金恢复会超出金币上限")
				return x
			}
			next[id] = p
			recovered = true
		}
	}
	if recovered {
		if err = x.persist(next); err != nil {
			x.recovery = next
		} else {
			x.profiles = next
		}
	}
	return x
}
func (x *pokemonSocialStore) persist(next map[string]pokemonProfile) error {
	if x.loadErr != nil {
		return fmt.Errorf("联机档案读取失败，原文件保留：%w", x.loadErr)
	}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if len(data) > 64<<20 {
		return errors.New("联机档案空间已满")
	}
	if err = os.MkdirAll(filepath.Dir(x.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(x.path), ".pokemon-social-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), x.path)
}
func (x *pokemonSocialStore) copyProfiles() map[string]pokemonProfile {
	next := make(map[string]pokemonProfile, len(x.profiles))
	for id, p := range x.profiles {
		next[id] = p
	}
	return next
}
func (s *Server) pokemonProfileEvent(id string, p pokemonProfile, note string) {
	s.sendToSocial(id, map[string]any{"type": "pokemon_profile", "data": map[string]any{"state": p.State, "revision": p.Revision, "note": note}})
}
func (x *pokemonSocialStore) occupied(id string) bool {
	if p := x.profiles[id]; p.Escrow {
		return true
	}
	if x.matches[id] != nil {
		return true
	}
	for _, o := range x.offers {
		if o.From == id || o.To == id {
			return true
		}
	}
	return false
}
func (s *Server) handlePokemonSocial(peer *socketPeer, r socialRequest) bool {
	if !strings.HasPrefix(r.Type, "pokemon_") {
		return false
	}
	if peer.ctx.Err() != nil || !s.pokemonOnline(peer.playerID) {
		_ = peer.send(map[string]any{"type": "pokemon_error", "error": "请先进入宝可梦模式，再使用联机中心"})
		return true
	}
	x := s.pokemonSocial
	x.mu.Lock()
	defer func() {
		changed := s.refreshPokemonActivitiesLocked()
		x.mu.Unlock()
		if changed {
			s.broadcastPokemonPresence()
		}
	}()
	fail := func(err error) { _ = peer.send(map[string]any{"type": "pokemon_error", "error": err.Error()}) }
	if len(r.Data) > 2<<20 {
		fail(errors.New("联机档案超过2MB，未导入"))
		return true
	}
	if x.loadErr != nil {
		fail(fmt.Errorf("联机档案读取失败，原文件未覆盖：%w", x.loadErr))
		return true
	}
	if x.recovery != nil {
		fail(errors.New("服务器正在恢复联机押金，保存成功后自动恢复联机操作"))
		return true
	}
	s.expirePokemonOffersLocked()
	var data struct {
		State    pokemon.State      `json:"state"`
		Revision uint64             `json:"revision"`
		ID       string             `json:"id"`
		Kind     string             `json:"kind"`
		Slots    [2]int             `json:"slots"`
		Action   pokemon.DuelAction `json:"action"`
	}
	if len(r.Data) > 0 && json.Unmarshal(r.Data, &data) != nil {
		fail(errors.New("宝可梦交互数据无效"))
		return true
	}
	id := peer.playerID
	if !s.pokemonOnline(id) {
		fail(errors.New("你已离开宝可梦模式"))
		return true
	}
	switch r.Type {
	case "pokemon_pause":
		if x.occupied(id) {
			fail(errors.New("邀请或对战进行中，不能恢复本地冒险"))
			return true
		}
		x.ready[id] = false
		_ = peer.send(map[string]any{"type": "pokemon_paused", "text": "已退出联机中心，可以继续本地冒险"})
	case "pokemon_fetch":
		if p, ok := x.profiles[id]; ok {
			x.ready[id] = true
			s.pokemonProfileEvent(id, p, "已读取当前服务端联机档案")
			if m := x.matches[id]; m != nil {
				s.sendPokemonDuelLocked(m)
			}
		} else {
			fail(errors.New("当前没有服务端档案，请先同步联机中心"))
		}
	case "pokemon_sync":
		if x.occupied(id) {
			if p, ok := x.profiles[id]; ok {
				s.pokemonProfileEvent(id, p, "联机交互进行中，已恢复锁定档案")
			}
			if m := x.matches[id]; m != nil {
				s.sendPokemonDuelLocked(m)
			}
			return true
		}
		x.ready[id] = false
		if p, ok := x.profiles[id]; ok && p.Revision != data.Revision {
			x.ready[id] = true
			s.pokemonProfileEvent(id, p, "服务端联机结算版本更新，已恢复服务端队伍与金币")
			return true
		}
		if data.State.Name != peer.name || data.State.Battle != nil || data.State.Capture != nil || len(data.State.Party) == 0 {
			fail(errors.New("需领取初始伙伴并结束本地战斗后再连接联机中心"))
			return true
		}
		if err := pokemon.NormalizeData(&data.State); err != nil {
			fail(err)
			return true
		}
		if _, exists := x.profiles[id]; !exists && len(x.profiles) >= 8192 {
			fail(errors.New("服务器联机档案数量已满"))
			return true
		}
		next := x.copyProfiles()
		revision := uint64(1)
		if old, ok := x.profiles[id]; ok {
			revision = old.Revision + 1
		}
		p := pokemonProfile{State: pokemon.CloneState(data.State), Revision: revision}
		p.State.MultiplayerRevision = p.Revision
		next[id] = p
		if err := x.persist(next); err != nil {
			fail(err)
			return true
		}
		x.profiles = next
		x.ready[id] = true
		s.pokemonProfileEvent(id, p, "队伍与金币已同步，联机中心就绪")
	case "pokemon_roster":
		rows := []map[string]any{}
		for _, online := range s.socialPeers() {
			if online.playerID == id || s.socialMode(online.playerID) != table.PokemonMode {
				continue
			}
			if p, ok := x.profiles[online.playerID]; ok {
				rows = append(rows, map[string]any{"playerId": online.playerID, "name": online.name, "party": p.State.Party, "busy": x.occupied(online.playerID), "ready": x.ready[online.playerID], "money": p.State.Money})
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i]["playerId"].(string) < rows[j]["playerId"].(string) })
		_ = peer.send(map[string]any{"type": "pokemon_roster", "data": rows})
	case "pokemon_invite":
		a, aok := x.profiles[id]
		b, bok := x.profiles[r.TargetID]
		if r.TargetID == id || !aok || !bok || !s.pokemonOnline(r.TargetID) || !x.ready[id] || !x.ready[r.TargetID] {
			fail(errors.New("双方需要在线并先同步宝可梦联机档案"))
			return true
		}
		if x.occupied(id) || x.occupied(r.TargetID) {
			fail(errors.New("一方已有邀请或对战，请先完成或取消"))
			return true
		}
		if a.State.Battle != nil || b.State.Battle != nil {
			fail(errors.New("请先结束本地战斗"))
			return true
		}
		if data.Kind != "duel" && data.Kind != "trade" {
			fail(errors.New("请选择对战或交换"))
			return true
		}
		o := pokemonOffer{ID: fmt.Sprintf("%s-%d", id, time.Now().UnixNano()), Kind: data.Kind, From: id, To: r.TargetID, FromName: peer.name, ToName: b.State.Name, Slots: data.Slots, Expires: time.Now().Add(90 * time.Second).Unix()}
		if data.Kind == "duel" {
			if a.State.Money < 300 || b.State.Money < 300 || a.State.Money > 999999700 || b.State.Money > 999999700 {
				fail(errors.New("双方需至少300金币，并为奖励留出余额空间"))
				return true
			}
		} else {
			if data.Slots[0] < 0 || data.Slots[0] >= len(a.State.Party) || data.Slots[1] < 0 || data.Slots[1] >= len(b.State.Party) {
				fail(errors.New("交换伙伴编号无效"))
				return true
			}
			o.Monsters = [2]pokemon.Monster{a.State.Party[data.Slots[0]], b.State.Party[data.Slots[1]]}
		}
		x.offers[o.ID] = o
		for _, target := range []string{o.From, o.To} {
			s.sendToSocial(target, map[string]any{"type": "pokemon_offer", "data": o})
		}
	case "pokemon_reply":
		o, ok := x.offers[data.ID]
		if !ok {
			fail(errors.New("邀请已过期或取消"))
			return true
		}
		if id != o.To {
			fail(errors.New("只有被邀请者可以接受"))
			return true
		}
		if !s.pokemonOnline(o.From) || !s.pokemonOnline(o.To) || !x.ready[o.From] || !x.ready[o.To] {
			delete(x.offers, o.ID)
			fail(errors.New("对方已离线或离开模式，邀请取消"))
			return true
		}
		next := x.copyProfiles()
		a, b := next[o.From], next[o.To]
		if o.Kind == "trade" {
			// Trading changes party slices and Pokédex maps: clone only these two
			// profiles, leaving all unrelated adventures shared and immutable.
			a.State = pokemon.CloneState(a.State)
			b.State = pokemon.CloneState(b.State)
			if o.Slots[0] < 0 || o.Slots[0] >= len(a.State.Party) || o.Slots[1] < 0 || o.Slots[1] >= len(b.State.Party) || a.State.Party[o.Slots[0]] != o.Monsters[0] || b.State.Party[o.Slots[1]] != o.Monsters[1] {
				delete(x.offers, o.ID)
				fail(errors.New("交换队伍已变化，请重新邀请"))
				return true
			}
			ga, gb := pokemon.New(a.State.Name), pokemon.New(b.State.Name)
			ga.State = a.State
			gb.State = b.State
			if err := ga.ReceivedTrade(o.Slots[0], o.Monsters[1]); err != nil {
				fail(err)
				return true
			}
			if err := gb.ReceivedTrade(o.Slots[1], o.Monsters[0]); err != nil {
				fail(err)
				return true
			}
			a.State, b.State = ga.State, gb.State
			a.Revision++
			b.Revision++
			a.State.MultiplayerRevision = a.Revision
			b.State.MultiplayerRevision = b.Revision
			next[o.From], next[o.To] = a, b
			if err := x.persist(next); err != nil {
				fail(err)
				return true
			}
			x.profiles = next
			delete(x.offers, o.ID)
			s.pokemonProfileEvent(o.From, a, "双方确认，伙伴交换成功（通信进化已结算）")
			s.pokemonProfileEvent(o.To, b, "双方确认，伙伴交换成功（通信进化已结算）")
			for _, target := range []string{o.From, o.To} {
				s.sendToSocial(target, map[string]any{"type": "pokemon_offer_done", "text": "交换完成"})
			}
		} else {
			if a.State.Money < 300 || b.State.Money < 300 {
				fail(errors.New("双方金币不足，无法开始"))
				return true
			}
			duel, err := pokemon.NewDuel([2]string{o.FromName, o.ToName}, [2][]pokemon.Monster{a.State.Party, b.State.Party})
			if err != nil {
				fail(err)
				return true
			}
			a.State.Money -= 300
			b.State.Money -= 300
			a.Escrow = true
			b.Escrow = true
			a.Revision++
			b.Revision++
			a.State.MultiplayerRevision = a.Revision
			b.State.MultiplayerRevision = b.Revision
			next[o.From], next[o.To] = a, b
			if err = x.persist(next); err != nil {
				fail(err)
				return true
			}
			x.profiles = next
			delete(x.offers, o.ID)
			match := &pokemonMatch{IDs: [2]string{o.From, o.To}, Duel: duel, Updated: time.Now()}
			x.matches[o.From] = match
			x.matches[o.To] = match
			s.pokemonProfileEvent(o.From, a, "300金币已押入本场对战")
			s.pokemonProfileEvent(o.To, b, "300金币已押入本场对战")
			s.sendPokemonDuelLocked(match)
		}
	case "pokemon_cancel":
		if match := x.matches[id]; match != nil {
			if match.Duel.Winner == -2 {
				match.Duel.Winner = 0
				if match.IDs[0] == id {
					match.Duel.Winner = 1
				}
				match.Duel.Log = append(match.Duel.Log, "一方主动退出，按认输结算。")
			}
			if err := s.settlePokemonDuelLocked(match); err != nil {
				fail(err)
			} else {
				_ = peer.send(map[string]any{"type": "pokemon_offer_done", "text": "对战已按当前结果结算"})
			}
			return true
		}
		for key, o := range x.offers {
			if o.From == id || o.To == id {
				delete(x.offers, key)
				for _, target := range []string{o.From, o.To} {
					s.sendToSocial(target, map[string]any{"type": "pokemon_offer_done", "text": "邀请已拒绝或取消"})
				}
			}
		}
		_ = peer.send(map[string]any{"type": "pokemon_offer_done", "text": "当前邀请已取消"})
	case "pokemon_action":
		match := x.matches[id]
		if match == nil {
			fail(errors.New("当前没有玩家对战"))
			return true
		}
		if match.Duel.Winner != -2 {
			if err := s.settlePokemonDuelLocked(match); err != nil {
				fail(err)
			}
			return true
		}
		seat := 0
		if match.IDs[1] == id {
			seat = 1
		}
		if err := match.Duel.Submit(seat, data.Action); err != nil {
			fail(err)
			return true
		}
		match.Updated = time.Now()
		if match.Duel.Winner != -2 {
			if err := s.settlePokemonDuelLocked(match); err != nil {
				fail(err)
			}
		} else {
			s.sendPokemonDuelLocked(match)
		}
	default:
		return false
	}
	return true
}
func (s *Server) sendPokemonDuelLocked(m *pokemonMatch) {
	for seat, id := range m.IDs {
		s.sendToSocial(id, map[string]any{"type": "pokemon_duel", "data": m.Duel.View(seat)})
	}
}
func (s *Server) settlePokemonDuelLocked(m *pokemonMatch) error {
	if m.Duel.Winner == -2 {
		return errors.New("对战尚未结束，不能结算")
	}
	x := s.pokemonSocial
	if x.matches[m.IDs[0]] != m || x.matches[m.IDs[1]] != m {
		return nil
	}
	// Persist a decided outcome before releasing escrow. Save failures keep the
	// result immutable, and a restart resumes payout rather than refunding it.
	needsDecision := false
	for _, id := range m.IDs {
		if x.profiles[id].Resolution == "" {
			needsDecision = true
		}
	}
	if needsDecision {
		decision := x.copyProfiles()
		for seat, id := range m.IDs {
			p, ok := decision[id]
			if !ok || !p.Escrow {
				return errors.New("本场押金记录缺失")
			}
			p.Resolution = "loss"
			if m.Duel.Winner == -1 {
				p.Resolution = "refund"
			} else if m.Duel.Winner == seat {
				p.Resolution = "win"
			}
			decision[id] = p
		}
		if err := x.persist(decision); err != nil {
			return fmt.Errorf("结果保存失败，押金保留，后台将重试：%w", err)
		}
		x.profiles = decision
	}
	next := x.copyProfiles()
	for _, id := range m.IDs {
		p := next[id]
		if !p.Escrow {
			return errors.New("本场押金已结算")
		}
		switch p.Resolution {
		case "win":
			p.State.Money += 600
		case "refund":
			p.State.Money += 300
		case "loss":
		default:
			return errors.New("本场结果记录无效")
		}
		p.Escrow = false
		p.Resolution = ""
		p.Revision++
		p.State.MultiplayerRevision = p.Revision
		next[id] = p
	}
	if err := x.persist(next); err != nil {
		return fmt.Errorf("结算保存失败，押金仍保留，可重试：%w", err)
	}
	x.profiles = next
	for seat, id := range m.IDs {
		note := "平局，退还300金币"
		if m.Duel.Winner >= 0 {
			note = "本场失败，净损失300金币"
			if m.Duel.Winner == seat {
				note = "本场获胜，净获得300金币"
			}
		}
		s.pokemonProfileEvent(id, next[id], note)
	}
	s.sendPokemonDuelLocked(m)
	for _, id := range m.IDs {
		delete(x.matches, id)
	}
	return nil
}
func (s *Server) expirePokemonOffersLocked() {
	x := s.pokemonSocial
	now := time.Now().Unix()
	for key, o := range x.offers {
		if o.Expires <= now || !s.pokemonOnline(o.From) || !s.pokemonOnline(o.To) {
			delete(x.offers, key)
			for _, id := range []string{o.From, o.To} {
				s.sendToSocial(id, map[string]any{"type": "pokemon_offer_done", "text": "邀请已过期或一方已离开宝可梦模式"})
			}
		}
	}
	seen := map[*pokemonMatch]bool{}
	for _, m := range x.matches {
		if seen[m] {
			continue
		}
		seen[m] = true
		if m.Duel.Winner != -2 {
			_ = s.settlePokemonDuelLocked(m)
			continue
		}
		if time.Since(m.Updated) > 3*time.Minute {
			m.Duel.Winner = -1
			if m.Duel.Pending[0] != nil && m.Duel.Pending[1] == nil {
				m.Duel.Winner = 0
			}
			if m.Duel.Pending[1] != nil && m.Duel.Pending[0] == nil {
				m.Duel.Winner = 1
			}
			m.Duel.Log = append(m.Duel.Log, "连续3分钟未操作：一方已提交则判对手超时认输，双方未提交则平局退款。")
			_ = s.settlePokemonDuelLocked(m)
		}
	}
}

func (s *Server) pokemonOnline(id string) bool {
	return s.hasSocialPlayer(id) && s.socialMode(id) == table.PokemonMode
}
func (s *Server) pokemonMaintenanceLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.pokemonSocial.mu.Lock()
			if x := s.pokemonSocial; x.loadErr == nil && x.recovery != nil {
				if x.persist(x.recovery) == nil {
					x.profiles = x.recovery
					x.recovery = nil
				}
			}
			if s.pokemonSocial.loadErr == nil && s.pokemonSocial.recovery == nil {
				s.expirePokemonOffersLocked()
			}
			changed := s.refreshPokemonActivitiesLocked()
			s.pokemonSocial.mu.Unlock()
			if changed {
				s.broadcastPokemonPresence()
			}
		}
	}
}
func (s *Server) pokemonDisconnected(id string) {
	// Shutdown cancels transport for every player; leave escrow unresolved so
	// startup recovery refunds unfinished games instead of awarding whoever's
	// socket happened to close last.
	select {
	case <-s.stop:
		return
	default:
	}
	x := s.pokemonSocial
	x.mu.Lock()
	defer func() {
		changed := s.refreshPokemonActivitiesLocked()
		x.mu.Unlock()
		if changed {
			s.broadcastPokemonPresence()
		}
	}()
	delete(x.ready, id)
	if m := x.matches[id]; m != nil {
		if m.Duel.Winner == -2 {
			m.Duel.Winner = 0
			if m.IDs[0] == id {
				m.Duel.Winner = 1
			}
			m.Duel.Log = append(m.Duel.Log, "一方断开联机连接或离开模式，按认输结算。")
		}
		_ = s.settlePokemonDuelLocked(m)
	}
	for key, o := range x.offers {
		if o.From == id || o.To == id {
			delete(x.offers, key)
			other := o.From
			if other == id {
				other = o.To
			}
			s.sendToSocial(other, map[string]any{"type": "pokemon_offer_done", "text": "对方已离线，邀请取消"})
		}
	}
}

// The Pokémon store may acquire social.mu, but never opMu. Publish after
// releasing the store lock so room operations and status updates cannot deadlock.
func (s *Server) refreshPokemonActivitiesLocked() bool {
	x := s.pokemonSocial
	activities := map[string]string{}
	for _, peer := range s.socialPeers() {
		if peer.ctx.Err() != nil || s.socialMode(peer.playerID) != table.PokemonMode {
			continue
		}
		status := "adventuring"
		if x.ready[peer.playerID] {
			status = "center"
		}
		if x.profiles[peer.playerID].Escrow {
			status = "settling"
		}
		if m := x.matches[peer.playerID]; m != nil {
			status = "battling"
			if m.Duel.Winner != -2 {
				status = "settling"
			}
		} else {
			for _, offer := range x.offers {
				if offer.From == peer.playerID || offer.To == peer.playerID {
					status = "invited"
					if offer.Kind == "trade" {
						status = "trading"
					}
					break
				}
			}
		}
		activities[peer.playerID] = status
	}
	s.social.mu.Lock()
	defer s.social.mu.Unlock()
	changed := len(activities) != len(s.social.activities)
	if !changed {
		for id, status := range activities {
			if s.social.activities[id] != status {
				changed = true
				break
			}
		}
	}
	s.social.activities = activities
	return changed
}
func (s *Server) broadcastPokemonPresence() {
	s.opMu.Lock()
	s.broadcastPresenceLocked()
	s.opMu.Unlock()
}
