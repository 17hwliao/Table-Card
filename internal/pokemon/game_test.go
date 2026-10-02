package pokemon

import (
	"math"
	"reflect"
	"testing"
)

func winningGame() *Game {
	g := New("adventure test")
	g.Command("选择 杰尼龟")
	g.Party[0] = newMonster(151, 100)
	return g
}
func winBattle(t *testing.T, g *Game) {
	t.Helper()
	for i := 0; g.Battle != nil && i < 12; i++ {
		g.Battle.Foes[g.Battle.Enemy].HP = 1
		g.Party[g.Battle.Active].HP = g.Party[g.Battle.Active].MaxHP()
		g.Party[g.Battle.Active].Status = ""
		g.Party[g.Battle.Active].PP[1] = Moves[g.Party[g.Battle.Active].Moves[1]].PP
		if err := g.Command("招式 2"); err != nil {
			t.Fatal(err)
		}
	}
	if g.Battle != nil {
		t.Fatal("fixture battle did not finish")
	}
}
func TestFullKantoStoryAndChampionSave(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	g := winningGame()
	for area := 0; area <= 10; area++ {
		if g.Area != area {
			t.Fatalf("unexpected chapter %d", g.Area)
		}
		if err := g.Command("前进"); err == nil {
			t.Fatal("chapter gate bypassed")
		}
		for range 6 {
			if err := g.Command("挑战"); err != nil {
				t.Fatal(err)
			}
			winBattle(t, g)
		}
		if g.TrainerWins[area] != 6 {
			t.Fatal("six victories not counted")
		}
		if err := g.Command("挑战"); err == nil {
			t.Fatal("repeated challenge counted")
		}
		if err := g.Command("剧情"); err != nil {
			t.Fatal(err)
		}
		winBattle(t, g)
		if area == 5 {
			g.Command("前往 紫苑镇")
			g.Command("剧情")
			winBattle(t, g)
			g.Command("前往 玉虹市")
		}
		if Areas[area].Boss != "" {
			if err := g.Command("道馆"); err != nil {
				t.Fatal(err)
			}
			winBattle(t, g)
			if err := g.Command("道馆"); err == nil {
				t.Fatal("duplicate badge awarded")
			}
		}
		if area == 10 {
			for range 5 {
				if err := g.Command("联盟"); err != nil {
					t.Fatal(err)
				}
				winBattle(t, g)
			}
		}
		if err := g.Command("前进"); err != nil {
			t.Fatal(err)
		}
	}
	if !g.Completed || len(g.Badges) != 8 || g.Area != 11 || g.Items["大师球"] != 1 {
		t.Fatal("incomplete champion adventure")
	}
	if err := g.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(g.Name)
	if err != nil || !reflect.DeepEqual(loaded.State, g.State) {
		t.Fatalf("champion save failed: %v", err)
	}
	g.Command("传说 超梦")
	for i := range g.Party {
		g.Party[i].HP = 0
	}
	g.endTurn()
	if g.Battle != nil {
		t.Fatal("blackout did not end the battle")
	}
	if !g.Completed || g.Elite != 5 {
		t.Fatal("postgame blackout corrupted champion flags")
	}
	if err := g.Save(); err != nil {
		t.Fatal(err)
	}
}
func TestThreeShakesAndPendingCaptureSave(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	g := winningGame()
	g.Command("进草丛")
	if err := g.Command("捕捉 精灵球"); err != nil {
		t.Fatal(err)
	}
	g.Capture.Passed = [3]bool{true, true, true}
	g.AdvanceCapture()
	if g.Capture.Step != 1 || len(g.Party) != 1 {
		t.Fatal("captured before three shakes")
	}
	if err := g.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(g.Name)
	if err != nil || loaded.Capture.Passed != g.Capture.Passed || loaded.Capture.Step != 1 {
		t.Fatal("capture rerolled on load")
	}
	loaded.AdvanceCapture()
	loaded.AdvanceCapture()
	if loaded.Capture != nil || loaded.Battle != nil || len(loaded.Party) != 2 {
		t.Fatal("third shake did not capture")
	}
	g = winningGame()
	g.Command("进草丛")
	g.Command("捕捉 精灵球")
	g.Capture.Passed = [3]bool{false, true, true}
	g.AdvanceCapture()
	if g.Capture != nil || g.Battle == nil || g.Battle.Turns != 1 || g.Items["精灵球"] != 9 {
		t.Fatal("failed capture not charged as a turn")
	}
}
func TestCaptureTotalsAgainstEnumeratedRandomDraws(t *testing.T) {
	for _, ball := range []string{"精灵球", "超级球", "高级球"} {
		for _, status := range []string{"", "睡眠", "麻痹"} {
			m := newMonster(25, 20)
			m.Status = status
			a, b, s := 255, 12, 0
			if ball == "超级球" {
				a, b = 200, 8
			}
			if ball == "高级球" {
				a = 150
			}
			if status == "睡眠" {
				s = 25
			}
			if status == "麻痹" {
				s = 12
			}
			w := (m.MaxHP() * 255 / b) / max(1, m.HP/4)
			success := 0
			for r1 := 0; r1 <= a; r1++ {
				for r2 := 0; r2 <= 255; r2++ {
					if r1 < s || (r1-s <= Dex[m.Species].CatchRate && r2 <= w) {
						success++
					}
				}
			}
			expected := float64(success) / float64((a+1)*256)
			if math.Abs(CaptureProbability(m, ball)-expected) > 1e-12 {
				t.Fatal("capture total differs from Gen I random draws")
			}
		}
	}
}
func TestShopPartyEvolutionAndInvalidInput(t *testing.T) {
	g := winningGame()
	money := g.Money
	if err := g.Command("购买 精灵球 -1"); err == nil || g.Money != money {
		t.Fatal("negative purchase changed money")
	}
	g.Command("购买 精灵球 5")
	if g.Items["精灵球"] != 15 {
		t.Fatal("purchase failed")
	}
	g.Party = append(g.Party, newMonster(25, 10))
	g.Items["雷之石"] = 1
	if err := g.Command("进化 雷之石 2"); err != nil || g.Party[1].Species != 26 {
		t.Fatal("stone evolution failed")
	}
	g.Command("存入 2")
	g.Command("取出 1")
	if len(g.Party) != 2 || len(g.Box) != 0 {
		t.Fatal("PC management failed")
	}
	for i := 0; i < 250; i++ {
		g.Notice("记录")
	}
	if len(g.Log) != 120 {
		t.Fatal("history is not bounded")
	}
	if err := validate(g.State); err != nil {
		t.Fatal(err)
	}
}
