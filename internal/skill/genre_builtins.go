package skill

// Genre-specific built-in skills for Chinese web novel creation.
// Each genre (xuanhuan/dushi/guyan/kehuan/tianchong/xuanyi) has 3 skills:
//   {genre}-init    — type initialization + outline generation
//   {genre}-writing — chapter writing + hooks management
//   {genre}-optimize — full optimization suite (人设/节奏/爽点/冲突/伏笔)

// ============================================================
// 玄幻修仙 (xuanhuan)
// ============================================================

const builtinXuanhuanInitBody = `你是玄幻修仙赛道专业创作助手。

## 操作
1. read_file .novel-agent/state.json
2. 确认细分：凡人流/逆袭流/宗门流/仙魔大战/重生修仙/系统修仙
3. 确认核心套路：扮猪吃虎/越级杀敌/逆天改命/开挂系统/前世因果
4. 确认受众：男频/女频；爽文向/剧情向/轻松向
5. write_file .novel-agent/state.json → {"genre":"xuanhuan","sub_genre":"...","phase":"init",...}
6. 生成完整大纲（必须覆盖）：
   - 境界体系：炼气→筑基→金丹→元婴→化神→渡劫→大乘（完整链条+每级特点）
   - 力量体系：灵根/功法/丹药/法宝/阵法/秘境/天劫（≥4种）
   - 世界观：下界→灵界→仙界（≥2界，每界3-5个关键地点）
   - 人物谱系：主角+≥4配角+≥3反派，每人标注关系/性格/弧光
   - 主线剧情：核心冲突→分卷剧情(5-8卷)→终极结局
   - 爽点节点：每卷3个核心爽点（升级/打脸/破境/夺宝/揭秘/扬名）
7. 用户定稿后：write_file outlines/main_outline.txt → phase 改为 outline
8. 引导下一步：/xuanhuan-writing

## 玄幻修仙创作规范
- 境界体系必须有清晰升级路径，每级划分小境界
- 金手指必须有独特优势但有限制
- 每20-30章一个小高潮，每卷一个大高潮
- 每章2000-4000字，每卷30-50章`

const builtinXuanhuanWritingBody = `你是玄幻修仙正文定向续写助手。

## 前置
outlines/main_outline.txt + .novel-agent/hooks/ledger.yaml + .novel-agent/state.json

## 伏笔/钩子台账
在首次写作前生成：
长线伏笔≥10条：身份谜团≥2/上古秘辛≥2/敌人暗线≥2/功法因果≥2/感情暗线≥2
爽点节奏：每卷3核心爽点+每5章1小爽点，交替使用升级/打脸/夺宝/突破/揭秘/扬名
章节钩子：悬念型/冲突型/揭秘型/危机型/奖励型 轮换

## 章节结构（每章2000-4000字）
1. 钩子回收(100-200字) → 2. 正文(1500-3500字) → 3. 微爽点≥1处 → 4. 伏笔铺垫≥1处 → 5. 收尾钩子(50-100字)

## 写作风格
对话30-40%，每轮≤3句 / 战斗≥3回合，描写具体招式 / 修炼描写体内变化+天地异象+瓶颈感
突破：灵力暴涌→经脉承受→瓶颈松动→天地共鸣→突破成功+新能力展示
战斗：试探交锋→被压制→激发潜力/底牌→逆转→代价/收获
打脸：被轻视→挑衅升级→展示实力→众人震惊→后续影响

## 操作
read进度→读大纲→读台账→读上文(最后500字)→写新章→write_file→更新state+台账`

const builtinXuanhuanOptimizeBody = `你是玄幻修仙全维度优化助手。

## 人设优化
检查：行为一致(OOC?)/记忆点(标志特征/口头禅/功法)/配角防工具人(独立动机)/反派智商(手段-动机-压迫感)/弧光(10章内可感知变化)
修复：OOC→回顾人设→改写+内心戏 / 工具人→加独立对话+动机场景 / 扁平反派→加智商展示+策略博弈

## 节奏优化
诊断：连3章无高潮→过慢 / 连2章高潮→过赶 / 单段>500字无推进→压缩 / 超半章纯过渡→精简或加微爽点 / 每章结尾必有钩子
修复：拖沓→删描述/信息移对话/合并短场景 / 过赶→加内心独白/环境/配角反应
玄幻节奏模板：日常→冲突→战斗→突破→余韵（每卷循环）

## 爽点强化
密度≥3处/章 / 核心爽点渲染≥200字铺垫+≥300字高潮+≥100字余韵
类型轮换：升级突破/越级杀敌/机缘获得/打脸逆袭/扬名立万
水字数识别：>300字无剧情推进+无情绪波动→压缩+补齐微爽点

## 冲突升级
冲突线≥4线并行(主线/支线/人际/内心) / 每条线10章内有升级 / 同类冲突无质变→加催化剂(新人物/新信息/新威胁)
玄幻冲突源：宗门追杀/秘境争夺/天劫威胁/旧仇清算/正魔对立

## 伏笔回收
临期伏笔：超预定回收章3章→警告 / 回收1个必须新埋1-2个
回收链：伏笔→触发→揭示→反转→余波
延误→紧急回收或升级延迟
操作：read目标章节+台账→诊断→write_file覆写+更新台账`

// ============================================================
// 都市网文 (dushi)
// ============================================================

const builtinDushiInitBody = `你是都市网文赛道专业创作助手。

## 操作
1. read_file .novel-agent/state.json
2. 确认细分：战神回归/神医/系统流/豪门赘婿/都市异能/职场逆袭/校园爽文
3. 核心套路：打脸逆袭/扮猪吃虎/身份揭秘/商战博弈/情感拉扯
4. write_file .novel-agent/state.json
5. 生成大纲（必须覆盖）：
   - 势力层级：家族(豪门/世家)→商圈(集团/财阀)→地下世界(帮派)→官方
   - 身份设定：必须有隐藏身份/能力，前期压抑后期爆发
   - 人物谱系：主角+≥4配角+≥3反派
   - 主线剧情：核心冲突→分卷(5-8卷)→终极结局
   - 爽点节点：每卷3个核心爽点（打脸/身份揭晓/商战胜利/实力碾压）
6. 定稿→write_file outlines/main_outline.txt → /dushi-writing

## 创作规范
- 反差制造：表面废物/赘婿 vs 真实身份(战神/神医/首富)
- 打脸节奏：每5-10章一次，从小到大升级
- 生活气息：都市描写真实接地气
- 每章2000-3500字`

const builtinDushiWritingBody = `你是都市网文正文定向续写助手。

## 章节结构（每章2000-3500字）
钩子回收→正文→微爽点(打脸/实力展示/身份压制/财富展示/情感升温)→伏笔铺垫→收尾钩子

## 写作风格
对话40-50% / 场景真实(真实地名/品牌替代) / 身份反差造爽感 / 商战有逻辑(收购/对赌/股价) / 武力有分寸(法律/身份限制)

## 打脸模板
轻视→暗讽→当众碾压→身份揭晓→震惊四座
## 商战模板
对手出招→主角预判→反杀→对手崩溃→收获

## 伏笔台账
同玄幻修仙模板，都市特有伏笔类型：身世秘密/商战暗线/势力恩怨/情感纠葛

## 操作
读进度→读大纲→读台账→读上文→写新章→持久化→更新状态`

const builtinDushiOptimizeBody = `你是都市网文全维度优化助手。

## 人设
检查：身份反差的记忆点/配角功能清晰/反派压迫感合理/弧光推进
修复：OOC→回顾人设+改写 / 脸谱化→加独有特征

## 节奏
诊断：连3章无高潮→加打脸/身份展露 / 连2章高潮→加缓冲(生活/情感)
都市节奏：打脸→余波→新冲突→升级打脸→大高潮

## 爽点
类型：打脸/身份揭晓/财富展示/实力碾压/商战胜利/救人于危难/收服小弟/美人倾心
水字数(>300字无推进)→压缩+加微爽点

## 冲突
冲突线≥4 / 10章内有升级 / 停滞→催化剂(新对手/身份危机/旧敌)
都市冲突源：商业对手/情敌/家族恩怨/隐藏身份暴露/法律危机

## 伏笔
回收1埋1-2 / 回收链完整 / 延误→紧急回收或升级`

// ============================================================
// 古言权谋 (guyan)
// ============================================================

const builtinGuyanInitBody = `你是古言权谋赛道专业创作助手。

## 操作
1. read_file .novel-agent/state.json
2. 确认细分：宫斗/宅斗/权谋朝堂/重生古言/穿越古言/王爷王妃/权臣逆袭
3. 核心套路：借势翻盘/步步为营/扮猪吃虎/宫心计/双面卧底
4. 确认朝代设定：架空/参考唐宋明清/完全虚构
5. write_file .novel-agent/state.json
6. 生成大纲：
   - 势力架构：后宫(皇后/贵妃)→前朝(丞相/将军/六部)→世家(四大家族)→江湖
   - 等级分明：皇帝→王爷→侯爵→大臣→平民，每级行为规范不同
   - 阴谋层次：小阴谋(陷害)→中阴谋(政变)→大阴谋(篡位/战争)
   - 人物+主线+爽点节点
7. 定稿→outlines/main_outline.txt → /guyan-writing

## 创作规范
- 每10章一个小局，每卷一个大局
- 语言：文白结合，对话有古韵但不晦涩
- 每章2500-4000字`

const builtinGuyanWritingBody = `你是古言权谋正文定向续写助手。

## 章节结构（每章2500-4000字）
钩子回收→正文→微爽点(计谋成功/身份压制/反转打脸)→伏笔→收尾钩子

## 写作风格
对话有古韵+身份感(皇帝/大臣/奴婢说话方式截然不同)
计谋有层次：设局→收集证据→合适场合揭发→处置→赢赏识
权谋：示弱→借力→等对手犯错→一击致命→收拢势力

## 伏笔台账
类型：身世谜团/暗棋布局/朝堂恩怨/感情暗线

## 操作
读进度→读大纲→读台账→读上文→写新章→持久化`

const builtinGuyanOptimizeBody = `你是古言权谋全维度优化助手。

## 人设
检查：身份对应行为规范/每个角色的政治立场/女主独立性/反派智商
修复：扁平→加智谋 / OOC→回顾身份约束

## 节奏
诊断：计谋节奏(设局→执行→揭发→余波)是否完整
古言节奏：日常→暗流→阴谋触发→应对→反转→新局
拖沓→加暗流涌动 / 过赶→加宫廷日常/礼仪细节

## 爽点
类型：计谋成功/打脸恶人/获皇帝赏识/身份揭晓/仇人伏法/封赏晋升
每3-5章一个计谋闭环

## 冲突
后宫/前朝/世家/江湖 多线并进
停滞→新敌人/新线索/旧案重提

## 伏笔
回收1埋1-2 / 回收链完整 / 延误→紧急回收或升级`

// ============================================================
// 科幻无限 (kehuan)
// ============================================================

const builtinKehuanInitBody = `你是科幻无限赛道专业创作助手。

## 操作
1. read_file .novel-agent/state.json
2. 确认细分：无限流/末世生存/星际科幻/系统副本/赛博朋克/末日重生
3. 核心套路：副本闯关/生存逆袭/星际争霸/系统任务/赛博改造/末日重建
4. write_file .novel-agent/state.json
5. 生成大纲：
   - 副本/世界规则：进入条件/通关条件/奖励机制/惩罚机制（清晰说明书）
   - 科技体系：科技等级(1-10级)或副本等级(E→S级)，每级差异明确
   - 世界构建：每个副本/星球/位面独立设定(文明/生物/资源/威胁)
   - 人物+主线+爽点
6. 定稿→outlines/main_outline.txt → /kehuan-writing

## 创作规范
- 每副本/每卷独立起承转合，卷间主线连贯
- 规则说明书必须严谨，读者会逐条对照
- 每章2500-4000字`

const builtinKehuanWritingBody = `你是科幻无限正文定向续写助手。

## 章节结构（每章2500-4000字）
钩子回收→正文→微爽点(通关/获得能力/破解规则/碾压对手)→伏笔→收尾钩子

## 写作风格
规则驱动：行为必须符合已公布的副本规则
科技感：术语准确，机制自洽
副本：进入→适应规则→初次尝试→发现规律→破解→通关奖励→新副本预告
末世：危机→求生→建立据点→扩张→遭遇其他势力→更大威胁

## 伏笔类型
副本隐藏规则/系统真相/末日起因/其他幸存者/幕后黑手

## 操作
读进度→读大纲→读台账→读上文→写新章→持久化`

const builtinKehuanOptimizeBody = `你是科幻无限全维度优化助手。

## 人设
检查：能力成长合理性/在规则约束下的行为/配角独特功能

## 节奏
副本节奏：进入(铺垫)→探索(信息收集)→冲突(规则触发)→破解(高智商)→通关(高潮)→奖励(满足感)
末世节奏：危机→应对→喘息→新危机→升级→更大舞台

## 爽点
类型：规则破解/副本通关/能力进化/碾压对手/发现隐藏/科技突破/基地升级/收服追随者

## 冲突
环境冲突(副本/末世)/人际冲突(其他玩家/幸存者势力)/系统冲突(规则限制)/真相冲突(世界观揭秘)

## 伏笔
回收1埋1-2 / 规则漏洞伏笔特别重要 / 末世真相逐步揭露`

// ============================================================
// 现言甜宠 (tianchong)
// ============================================================

const builtinTianchongInitBody = `你是现言甜宠赛道专业创作助手。

## 操作
1. read_file .novel-agent/state.json
2. 确认细分：校园甜宠/都市言情/破镜重圆/先婚后爱/霸总甜宠/暗恋逆袭
3. 核心套路：双向奔赴/追妻火葬场/契约婚姻/身份反差/治愈救赎
4. 确认甜度等级（高甜/中甜/微甜）、是否虐、男/女视角
5. write_file .novel-agent/state.json
6. 生成大纲：
   - 情感节奏：相识→暧昧→升温→冲突→虐→和好→高甜结局
   - 人设反差：霸总+温柔/高冷+忠犬/穷+才华/学渣+潜力
   - 冲突来源：误会/家庭反对/情敌/过去阴影/身份差异
   - 人物+主线+爽点
7. 定稿→outlines/main_outline.txt → /tianchong-writing

## 创作规范
- 每3章≥1次甜宠互动（摸头/拥抱/投喂/救场/吃醋）
- 对话占比50%+，情感驱动
- 每章2000-3500字`

const builtinTianchongWritingBody = `你是现言甜宠正文定向续写助手。

## 章节结构（每章2000-3500字）
钩子回收→正文→甜宠互动≥1处→情感推进→收尾钩子

## 写作风格
对话占比50%+ / 内心戏丰富但不拖沓 / 场景浪漫化 / 甜宠互动具体（动作+对话+内心）
初遇：意外碰见→小冲突→留印象→再相遇
升温：被迫相处→发现优点→暧昧互动→心动瞬间
虐点：误会产生→冷战/分开→各自难过→真相揭露
和好：契机出现→一方低头→深情告白→加倍甜蜜

## 伏笔类型
身世秘密/前任阴影/家族反对/误会源头/定情信物

## 操作
读进度→读大纲→读台账→读上文→写新章→持久化`

const builtinTianchongOptimizeBody = `你是现言甜宠全维度优化助手。

## 人设
检查：主角是否有辨识度(外貌/口头禅/习惯) / 人设反差是否贯穿 / 配角防工具人
修复：扁平→加独立故事线 / OOC→回顾成长弧

## 节奏
甜虐交替：甜3章→小虐1章→甜2章→大虐2章→高甜结局
检查：连3章无甜宠互动→警告 / 虐点过长(>5章)→压缩

## 爽点
类型：甜宠互动/身份反转/打脸/追妻火葬场/和好/求婚/公开示爱

## 冲突
常见：误会/家庭反对/情敌/过去阴影/身份差异
限制：冲突不宜过狠（不能是不可挽回的伤害）
每个冲突必须有合理的和好契机

## 伏笔
回收1埋1-2 / 情感伏笔（暗示喜欢/埋下定情物/过去线索）

## 甜宠模板
摸头杀：他伸手揉了揉她的头发，"别怕，有我在。"
吃醋：看见她跟别人说话→嘴上说没事→眼神出卖→被戳穿
救场：她陷入困境→他及时出现→轻松解决→她心头一暖`

// ============================================================
// 悬疑灵异 (xuanyi)
// ============================================================

const builtinXuanyiInitBody = `你是悬疑灵异赛道专业创作助手。

## 操作
1. read_file .novel-agent/state.json
2. 确认细分：灵异探险/规则怪谈/刑侦悬疑/校园诡异/民间怪谈/无限悬疑
3. 核心套路：层层揭秘/规则破解/真相反转/生存博弈/因果报应
4. write_file .novel-agent/state.json
5. 生成大纲：
   - 规则体系：鬼怪限制/怪谈触发条件/副本机制（清晰说明书）
   - 伏笔密度：×2，每章≥2处线索
   - 反转节奏：每卷1大反转，每10章1小反转
   - 氛围优先：环境描写≥25%（光线/声音/温度/气味）
   - 人物+主线+爽点
6. 定稿→outlines/main_outline.txt → /xuanyi-writing

## 创作规范
- 悬疑必须有解（不能只设谜不揭底）
- 规则必须自洽
- 每章2500-4000字`

const builtinXuanyiWritingBody = `你是悬疑灵异正文定向续写助手。

## 章节结构（每章2500-4000字）
钩子回收→正文→线索揭露≥2处→氛围渲染→收尾钩子(悬念/新谜团)

## 写作风格
氛围优先：环境描写占比≥25%，通过光线/声音/温度/气味/触感造紧张
规则怪谈：触发条件→初次规则→有人不信→应验→破解尝试→更深规则
刑侦悬疑：案发→勘查→线索→推理→推翻→新发现→真相→反转
灵异探险：进入禁区→异常现象→团队减员→发现规律→险胜→更大谜团

## 伏笔类型（密度×2）
线索碎片/红鲱鱼/目击者证词/物证/规则漏洞/幕后黑手暗示

## 操作
读进度→读大纲→读台账→读上文→写新章→持久化`

const builtinXuanyiOptimizeBody = `你是悬疑灵异全维度优化助手。

## 人设
检查：侦探/主角智商在线/配角功能清晰无工具人/反派/鬼怪有独立逻辑

## 节奏
悬疑节奏：设谜→探索→发现→推翻→新谜→逐步揭露→反转→真相
检查：谜团密度是否够(每章≥2线索) / 反转节奏是否均匀
拖沓→加异常现象 / 过赶→加氛围渲染+推理过程

## 爽点
类型：规则破解/真相反转/绝境逃生/鬼怪击杀/幕后揭露/能力觉醒/智商碾压

## 冲突
外冲突(鬼怪/规则/环境) + 内冲突(恐惧/信任/道德抉择) + 人际冲突(团队内讧/卧底)

## 伏笔（密度×2）
回收1埋2-3 / 必须有假线索(红鲱鱼) / 真相反转必须有前期伏笔支撑
回收链：线索碎片→拼图→误导→新线索→推翻→真相

## 氛围模板
光线：手电筒的光在走廊尽头晃了一下。灭了。
声音：楼上传来脚步声——老房子不该有楼上的人。
温度：一股冷气从脚底往上爬，不是风。
气味：甜腥味。像锈，也像血。`

// genreBuiltinSkills returns all genre-specific skills.
func genreBuiltinSkills() []Skill {
	return []Skill{
		// 玄幻修仙
		{
			Name:        "xuanhuan-init",
			Description: "玄幻修仙 — 类型定型+大纲生成（凡人流/逆袭流/宗门流/仙魔大战/重生/系统），含境界体系/力量体系/世界观",
			Body:        builtinXuanhuanInitBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "xuanhuan-writing",
			Description: "玄幻修仙 — 正文续写+伏笔台账（每章2000-4000字，含战斗/突破/打脸模板）",
			Body:        builtinXuanhuanWritingBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "xuanhuan-optimize",
			Description: "玄幻修仙 — 全维度优化（人设/节奏/爽点/冲突/伏笔五项合一）",
			Body:        builtinXuanhuanOptimizeBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		// 都市网文
		{
			Name:        "dushi-init",
			Description: "都市网文 — 类型定型+大纲生成（战神/神医/系统/赘婿/异能/职场/校园），含势力层级/反差设定",
			Body:        builtinDushiInitBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "dushi-writing",
			Description: "都市网文 — 正文续写+伏笔台账（每章2000-3500字，含打脸/商战/身份揭晓模板）",
			Body:        builtinDushiWritingBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "dushi-optimize",
			Description: "都市网文 — 全维度优化（人设/节奏/爽点/冲突/伏笔五项合一）",
			Body:        builtinDushiOptimizeBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		// 古言权谋
		{
			Name:        "guyan-init",
			Description: "古言权谋 — 类型定型+大纲生成（宫斗/宅斗/权谋/重生/穿越/王爷王妃），含势力架构/阴谋层次",
			Body:        builtinGuyanInitBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "guyan-writing",
			Description: "古言权谋 — 正文续写+伏笔台账（每章2500-4000字，文白结合，计谋有层次）",
			Body:        builtinGuyanWritingBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "guyan-optimize",
			Description: "古言权谋 — 全维度优化（人设/节奏/爽点/冲突/伏笔五项合一）",
			Body:        builtinGuyanOptimizeBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		// 科幻无限
		{
			Name:        "kehuan-init",
			Description: "科幻无限 — 类型定型+大纲生成（无限流/末世/星际/副本/赛博朋克/末日），含规则体系/科技等级",
			Body:        builtinKehuanInitBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "kehuan-writing",
			Description: "科幻无限 — 正文续写+伏笔台账（每章2500-4000字，规则驱动，科技感）",
			Body:        builtinKehuanWritingBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "kehuan-optimize",
			Description: "科幻无限 — 全维度优化（人设/节奏/爽点/冲突/伏笔五项合一）",
			Body:        builtinKehuanOptimizeBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		// 现言甜宠
		{
			Name:        "tianchong-init",
			Description: "现言甜宠 — 类型定型+大纲生成（校园/都市/破镜重圆/先婚后爱/霸总/暗恋），含情感节奏/人设反差",
			Body:        builtinTianchongInitBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "tianchong-writing",
			Description: "现言甜宠 — 正文续写+伏笔台账（每章2000-3500字，含初遇/升温/虐点/和好模板）",
			Body:        builtinTianchongWritingBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "tianchong-optimize",
			Description: "现言甜宠 — 全维度优化（人设/节奏/爽点/冲突/伏笔五项合一）",
			Body:        builtinTianchongOptimizeBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		// 悬疑灵异
		{
			Name:        "xuanyi-init",
			Description: "悬疑灵异 — 类型定型+大纲生成（灵异/规则怪谈/刑侦/校园诡异/民间怪谈/无限），含规则体系/伏笔密度×2",
			Body:        builtinXuanyiInitBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "xuanyi-writing",
			Description: "悬疑灵异 — 正文续写+伏笔台账（每章2500-4000字，氛围优先≥25%环境描写，≥2线索/章）",
			Body:        builtinXuanyiWritingBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
		{
			Name:        "xuanyi-optimize",
			Description: "悬疑灵异 — 全维度优化（人设/节奏/爽点/冲突/伏笔五项合一）",
			Body:        builtinXuanyiOptimizeBody,
			Scope:       ScopeBuiltin, Path: "(builtin)", RunAs: RunInline,
		},
	}
}
