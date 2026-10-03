package pokemon

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// These episodes and dialogue are original. They are optional and never
// replace the main story gates or count toward the six trainer victories.
type episode struct {
	Title, NPC, Offer, Clue, Ending string
	Foe                             int
}

var episodes = [...]episode{
	{"寄给明天的信", "邮差小满", "有人把明天的日期写在空信封上。小满担心是遗失的告别信，请你去博士后院找找收信人的线索。", "后院的风向标下藏着一张训练计划。所谓明天，是一个害怕出发的孩子给自己的约定。", "你们把信投进标着明天的邮筒。孩子决定先走到镇口；勇气有时只是一小段路。", 16},
	{"迷路的路标", "守林员阿苔", "森林里每块路标都指向同一棵树。阿苔怀疑有人恶作剧，却又发现树下每天多出一朵花。", "博物馆侧门的旧地图显示，那棵树曾是救护站。路标不是指错了路，而是在纪念帮助过旅人的地方。", "你和阿苔重画路标，保留一条通往老树的小径。再也没人迷路，也没人被遗忘。", 12},
	{"月亮的零件", "修表匠小刻", "小刻捡到一枚没有刻度的齿轮，坚称它来自月亮。他想把坏掉的钟修好，让研究员赶上回家的船。", "海角小屋里找到一块旧钟面，背面画着潮汐。齿轮来自山洞观测钟，月亮只是它一直追逐的指针。", "钟声重新响起。小刻把多余的齿轮做成吊坠，说每个人都有自己的时间。", 35},
	{"不肯离港的纸船", "见习船员阿舟", "一只纸船总漂回岸边。阿舟说那是他的第一次航海日志，可他一直没有勇气上真正的船。", "甲板上的防水盒装着另一半日志：船长年轻时也曾晕船。他给阿舟留了一场温和的练习对战。", "纸船终于顺流而去，阿舟登上甲板。你收到一张没有目的地的船票：出发本身就值得纪念。", 54},
	{"没有声音的铃", "修铃人小回", "小回修好的铃始终不响，但夜里总有伙伴在铃下睡着。他怕自己的手艺辜负了城镇。", "塔边石阶留下铃铛的旧铭文：它不是报时铃，而是旅人休息的记号。需要修复的是悬挂它的绳结。", "新绳结随风摇动。铃仍很轻，但小回终于听见了伙伴安心的呼吸。", 92},
	{"逆着开的花", "园艺师阿序", "花园里一株花朝地下开，小孩子们叫它坏掉的花。阿序想请你帮它找到合适的光。", "地下通道的裂缝透进一束反光。花追逐的是镜面中的阳光；旧镜片必须先从看守者手里取回。", "你们装好小小的反光板。阿序给花立了一块牌子：每一种生长，都有自己的方向。", 102},
	{"借来的脚印", "园长学徒小印", "保护区出现两排一模一样的脚印，小印担心有人偷偷带走野生伙伴。", "失物屋里找到一双练习用木鞋：一位受伤的旅人想陪伙伴散步，却不好意思请人帮忙。", "巡护队为旅人安排了缓缓前行的小车。小印把木鞋挂在门口，提醒自己先问故事，再作判断。", 111},
	{"电梯停在星期八", "电梯员阿层", "大楼的电梯多出一层名叫星期八。阿层每次按下按钮，都回到同一个没人使用的休息室。", "公司旧档案里有一份未寄出的休假申请。星期八是老维修员给大家留下的一天空白。", "你协助完成安全检修，电梯恢复正常。休息室保留下来；忙碌的城市也可以有暂停的一层。", 81},
	{"温度为零的火", "研究员小焰", "小焰的温度计测不出伙伴的火焰，他以为伙伴生病了。宅邸里或许保存着仪器说明。", "旧说明记录了一种玻璃隔热罩。温度计测的是罩外的空气；伙伴只是一直在保护实验者。", "隔热罩被换成安全的新支架。小焰第一次认真道谢，伙伴的火焰在夕阳下亮得很安静。", 77},
	{"还没开始的终点", "画地图的阿归", "阿归把地图上的终点画回镇口。他觉得这样所有离开的人都一定会回来，却担心旅人因此走不远。", "旧办公室里有许多寄回的明信片。归来的不只有脚步，也有远处的人仍愿意分享的日常。", "地图改成开放的环线。阿归在镇口添了一句话：这里欢迎出发，也欢迎想起。", 58},
	{"第九枚空徽章", "徽章匠小圆", "小圆做了一枚没有图案的徽章，被嘲笑没有资格成为奖品。他想找到它应当奖励的人。", "检验厅留着挑战失败者的登记册。他们一次次重新来过，空徽章或许该留给坚持而非胜负。", "小圆把徽章送给刚落败的孩子。你们约定下一次在联盟门口见面，无论结果如何。", 67},
	{"星星下班以后", "观测员阿夜", "阿夜担心成为冠军后，故事就没有下一页。他想弄清白天看不见的星星去了哪里。", "观测台的笔记写着：星星并没有离开，只是有些光需要等待。最后一场交流对战正在星图旁等你。", "你们在星图上留下十二枚旅途邮票。冠军不是句号；小小的相遇，把空白变成了下一页。", 148},
}

func sideFlag(area int, suffix string) string { return fmt.Sprintf("side-%d-%s", area, suffix) }
func (g *Game) QuestStage(area int) int {
	if g.Flags[sideFlag(area, "done")] {
		return 3
	}
	if g.Flags[sideFlag(area, "clue")] {
		return 2
	}
	if g.Flags[sideFlag(area, "accepted")] {
		return 1
	}
	return 0
}
func (g *Game) QuestCount() int {
	n := 0
	for i := range episodes {
		if g.QuestStage(i) == 3 {
			n++
		}
	}
	return n
}
func (g *Game) QuestInfo() string {
	e := episodes[g.Area]
	stage := []string{"尚未接受：选择一种回应。", "已接受：去建筑4调查现场寻找线索。", "线索齐全：选择解决委托对战。", "委托完成：可以继续主线或旅行。"}[g.QuestStage(g.Area)]
	text := fmt.Sprintf("《%s》 / %s\n%s\n%s\n旅途邮票 %d/12；支线不计入本章六场挑战。", e.Title, e.NPC, e.Offer, stage, g.QuestCount())
	if g.QuestStage(g.Area) == 3 {
		text = "《" + e.Title + "》 / 已完成\n" + e.Ending + fmt.Sprintf("\n旅途邮票 %d/12。", g.QuestCount())
	}
	if g.QuestCount() == 12 {
		text += "\n十二城的居民寄来一张拼接地图：所有出发的地方，都有一盏为你留着的灯。"
	}
	return text
}
func (g *Game) questCommand(arg string) error {
	stage := g.QuestStage(g.Area)
	e := episodes[g.Area]
	if arg == "" {
		g.say("%s", g.QuestInfo())
		return nil
	}
	if (arg == "1" || arg == "2") && stage == 0 {
		g.Flags[sideFlag(g.Area, "accepted")] = true
		g.Flags[sideFlag(g.Area, "choice-2")] = arg == "2"
		g.say("%s：%s", e.NPC, e.Offer)
		if arg == "1" {
			g.say("你坐下来听完了故事。对方在地图上圈出建筑4，向你认真道谢。")
		} else {
			g.say("你把手电与笔记放进背包，约好先到建筑4调查，再一起讨论发现。")
		}
		return nil
	}
	if arg == "3" && stage == 2 {
		return g.event(e.NPC+"·委托练习", []int{e.Foe}, max(5, Areas[g.Area].MinLevel), sideFlag(g.Area, "fight"))
	}
	return errors.New("当前阶段不可执行此选项；Q 打开委托菜单，或 quest 查看进度")
}
func (g *Game) questExplore() {
	if g.Location != 3 || g.QuestStage(g.Area) != 1 {
		return
	}
	g.Flags[sideFlag(g.Area, "clue")] = true
	g.say("调查发现：%s", episodes[g.Area].Clue)
	if g.Flags[sideFlag(g.Area, "choice-2")] {
		g.say("你把线索画成简图，对方终于理解了来龙去脉。")
	} else {
		g.say("先前听到的细节与线索吻合；对方愿意与你一起完成最后的练习。")
	}
	g.say("线索已记录，Q 打开委托菜单，选择解决委托对战。")
}
func (g *Game) sideReward(flag string) bool {
	parts := strings.Split(flag, "-")
	if len(parts) != 3 || parts[0] != "side" || parts[2] != "fight" {
		return false
	}
	area, err := strconv.Atoi(parts[1])
	if err != nil || area < 0 || area >= len(episodes) {
		return false
	}
	if g.QuestStage(area) == 3 {
		return true
	}
	g.Flags[sideFlag(area, "done")] = true
	reward := 300 + area*100
	g.Money = min(1_000_000_000, g.Money+reward)
	g.Items["伤药"] = min(999, g.Items["伤药"]+2)
	g.say("委托完成：%s", episodes[area].Ending)
	g.say("收到旅途邮票、$%d与伤药×2。邮票 %d/12。", reward, g.QuestCount())
	if len(g.Party) > 0 {
		g.gainExp(0, (area+1)*200)
	}
	if g.QuestCount() == 3 {
		g.say("三枚邮票解锁专属技教学资格；伙伴达到Lv20后，去城市中心学习。")
	}
	if g.QuestCount() == 12 {
		g.say("十二封回信拼成新的星图：愿你再次出发时，不再害怕故事的空白。原创支线篇章完结。")
	}
	return true
}

var chapterScenes = [...]string{
	"博士把空白地图推到桌边：地图替不了脚步。镇口的劲敌说，先认真打完六场，再谈远方。你听见伙伴轻轻应了一声。",
	"林间的训练者教你观察草叶的晃动。尼比博物馆的石头记着漫长岁月；小刚等待的，不只是更强的招式，还有理解伙伴的耐心。",
	"月见山的化石让你看见时间的另一面。海角小屋里，正辉的机器出了小故障；帮人修好归途，有时也会为自己打开下一扇门。",
	"枯叶港的汽笛在雾里回响。船长说旅行要学会照顾伙伴，马志士则提醒你：强大也意味着知道何时收住力量。",
	"紫苑的钟声比脚步轻。塔中的影子并非都想伤害旅人；先找到检视镜，再回来理解它们没能说出的故事。",
	"玉虹的花园与地下基地隔着几层石阶。莉佳邀请你看看花的生长，城市的另一面却需要你把被困的伙伴带回阳光。",
	"浅红的芦苇间有许多不必追赶的身影。园长遗失的金牙让大家忙成一团；找到它之后，海面与巨石都不再只是地图上的边界。",
	"金黄的楼层映着不同颜色的天空。西尔佛的研究员需要帮助，娜姿的沉默让你开始学习：看不见的压力，也需要被认真理解。",
	"红莲宅邸的手记写满被擦去的句子。你寻找钥匙，也思考实验留下的责任；夏伯用一场对战问你，热情能否成为温柔的光。",
	"常磐的路仍是出发时的路，你却已经走过了许多城市。关闭的道馆终于打开，坂木在这里接受一场没有借口的正式挑战。",
	"八枚徽章并排放在检验台上。联盟大厅的门一扇接着一扇，劲敌留下一句话：等你走到最后，我们再认真打一次。",
	"博士替你登记冠军记录，又递来一张空白调查表。城市里的支线、洞窟里的传说、未完成的图鉴，都在等下一次相遇。",
}

func (g *Game) chapterDialogue() {
	flag := fmt.Sprintf("chapter-dialogue-%d", g.Area)
	if !g.Flags[flag] {
		g.Flags[flag] = true
		g.say("章节序幕：%s", chapterScenes[g.Area])
	}
}

func signatureMove(species int) int { return 25 + species }
func signatureName(species int) string {
	switch species {
	case 1, 2, 3:
		return Dex[species].Name + "·翠庭回声"
	case 4, 5, 6:
		return Dex[species].Name + "·焰翼星火"
	case 7, 8, 9:
		return Dex[species].Name + "·海潮脉冲"
	case 25, 26:
		return Dex[species].Name + "·雷光跃动"
	case 133, 134, 135, 136:
		return Dex[species].Name + "·旅途之光"
	case 144, 145, 146, 150, 151:
		return Dex[species].Name + "·星界回响"
	default:
		return Dex[species].Name + "·回响"
	}
}
func (g *Game) trainSignature(index int) error {
	if index < 0 || index >= len(g.Party) {
		return errors.New("伙伴编号为1–6；P 查看队伍")
	}
	m := &g.Party[index]
	if m.Signature {
		return errors.New("这位伙伴已经学习过专属技，不会重复收费")
	}
	if m.Level < 20 || (len(g.Badges) < 2 && g.QuestCount() < 3) {
		return errors.New("需要Lv20，并持有2枚徽章或完成3个支线")
	}
	if g.Money < 500 {
		return errors.New("教学需要$500，当前零钱不足")
	}
	g.Money -= 500
	m.Signature = true
	g.refreshMoves(m)
	g.say("%s学会原创专属技%s，替换第四个招式。威力95 / 命中100 / PP8，属性与伙伴主属性一致。", m.Name(), signatureName(m.Species))
	return nil
}
