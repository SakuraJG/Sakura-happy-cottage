package service

import (
	"crypto/rand"
	"math/big"
	"strings"
)

type drawingWord struct {
	Word     string
	Category string
	Hint     string
	Aliases  []string
}

func (word drawingWord) matches(answer string) bool {
	answer = strings.TrimSpace(answer)
	if answer == word.Word {
		return true
	}
	for _, alias := range word.Aliases {
		if answer == alias {
			return true
		}
	}
	return false
}

var drawingWords = []drawingWord{
	{Word: "小猫", Category: "动物", Hint: "会发出轻柔叫声的宠物", Aliases: []string{"猫", "猫咪"}},
	{Word: "小狗", Category: "动物", Hint: "热情忠诚的宠物", Aliases: []string{"狗", "狗狗"}},
	{Word: "长颈鹿", Category: "动物", Hint: "脖子很长的草食动物"},
	{Word: "熊猫", Category: "动物", Hint: "黑白相间，喜欢竹子"},
	{Word: "企鹅", Category: "动物", Hint: "穿着黑白礼服的极地居民"},
	{Word: "兔子", Category: "动物", Hint: "长耳朵，擅长跳跃", Aliases: []string{"小兔子"}},
	{Word: "章鱼", Category: "动物", Hint: "生活在海里，有许多触手"},
	{Word: "海豚", Category: "动物", Hint: "聪明友善的海洋动物"},
	{Word: "袋鼠", Category: "动物", Hint: "腹部有育儿袋，跳得很远"},
	{Word: "松鼠", Category: "动物", Hint: "大尾巴，喜欢收集坚果"},
	{Word: "猫头鹰", Category: "动物", Hint: "夜晚活动，眼睛很大"},
	{Word: "金鱼", Category: "动物", Hint: "常养在鱼缸里的小动物"},
	{Word: "冰淇淋", Category: "美食", Hint: "夏天很受欢迎的冰凉甜品"},
	{Word: "汉堡包", Category: "美食", Hint: "面包中夹着肉和蔬菜", Aliases: []string{"汉堡"}},
	{Word: "小笼包", Category: "美食", Hint: "皮薄多汁的中式点心"},
	{Word: "西瓜", Category: "美食", Hint: "绿色外皮、红色果肉的水果"},
	{Word: "草莓", Category: "美食", Hint: "红色心形的小浆果"},
	{Word: "火锅", Category: "美食", Hint: "大家围着一口热锅吃"},
	{Word: "爆米花", Category: "美食", Hint: "看电影时常见的零食"},
	{Word: "蛋糕", Category: "美食", Hint: "庆祝生日时常会出现"},
	{Word: "胡萝卜", Category: "美食", Hint: "橙色的根茎蔬菜"},
	{Word: "荷包蛋", Category: "美食", Hint: "完整蛋黄在中间的煎蛋", Aliases: []string{"煎蛋"}},
	{Word: "棉花糖", Category: "美食", Hint: "蓬松柔软，像云朵一样"},
	{Word: "奶茶", Category: "美食", Hint: "茶和牛奶组合的饮料"},
	{Word: "雨伞", Category: "物品", Hint: "下雨天用来遮挡雨水", Aliases: []string{"伞"}},
	{Word: "书包", Category: "物品", Hint: "学生背着它去上学"},
	{Word: "闹钟", Category: "物品", Hint: "每天按时叫人起床"},
	{Word: "吉他", Category: "物品", Hint: "有六根弦的常见乐器"},
	{Word: "相机", Category: "物品", Hint: "用来记录画面的设备", Aliases: []string{"照相机"}},
	{Word: "眼镜", Category: "物品", Hint: "戴在眼前帮助看清东西"},
	{Word: "拖鞋", Category: "物品", Hint: "在家里穿的轻便鞋子"},
	{Word: "牙刷", Category: "物品", Hint: "每天早晚清洁牙齿"},
	{Word: "风筝", Category: "物品", Hint: "借助风在天空中飞"},
	{Word: "台灯", Category: "物品", Hint: "放在桌面上照亮书本"},
	{Word: "钥匙", Category: "物品", Hint: "可以打开特定的锁"},
	{Word: "热气球", Category: "物品", Hint: "靠热空气升上天空"},
	{Word: "小房子", Category: "地点自然", Hint: "温暖的住处", Aliases: []string{"房子", "家"}},
	{Word: "游乐园", Category: "地点自然", Hint: "有许多娱乐设施的地方"},
	{Word: "彩虹", Category: "地点自然", Hint: "雨后天空中的七彩弧线"},
	{Word: "火山", Category: "地点自然", Hint: "可能喷出岩浆的山"},
	{Word: "沙滩", Category: "地点自然", Hint: "海边铺满细沙的地方"},
	{Word: "雪人", Category: "地点自然", Hint: "冬天用积雪堆成的人形"},
	{Word: "月亮", Category: "地点自然", Hint: "夜空中明亮的天体"},
	{Word: "瀑布", Category: "地点自然", Hint: "从高处奔流而下的水"},
	{Word: "电影院", Category: "地点自然", Hint: "在大银幕前观看影片"},
	{Word: "图书馆", Category: "地点自然", Hint: "安静阅读和借书的地方"},
	{Word: "摩天轮", Category: "地点自然", Hint: "缓慢转动的巨大圆形设施"},
	{Word: "樱花树", Category: "地点自然", Hint: "春天会开粉色花朵的树"},
	{Word: "跳绳", Category: "动作职业", Hint: "让绳子反复从脚下经过"},
	{Word: "踢足球", Category: "动作职业", Hint: "用脚把球送进球门"},
	{Word: "睡觉", Category: "动作职业", Hint: "闭上眼睛休息很久"},
	{Word: "洗澡", Category: "动作职业", Hint: "用水清洁身体"},
	{Word: "拍照", Category: "动作职业", Hint: "按下快门记录画面"},
	{Word: "做饭", Category: "动作职业", Hint: "把食材变成一顿餐食"},
	{Word: "医生", Category: "动作职业", Hint: "在医院帮助病人康复"},
	{Word: "消防员", Category: "动作职业", Hint: "穿防护服扑灭火灾"},
	{Word: "宇航员", Category: "动作职业", Hint: "乘坐飞船探索太空"},
	{Word: "画家", Category: "动作职业", Hint: "用画笔创作图像的人"},
	{Word: "钓鱼", Category: "动作职业", Hint: "拿着鱼竿等待鱼儿上钩"},
	{Word: "骑自行车", Category: "动作职业", Hint: "踩踏板让两个轮子前进"},
}

func randomDrawingWord() (drawingWord, error) {
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(drawingWords))))
	if err != nil {
		return drawingWord{}, err
	}
	return drawingWords[index.Int64()], nil
}
