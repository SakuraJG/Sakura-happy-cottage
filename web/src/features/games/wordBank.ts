export type DrawingWord = {
  word: string
  category: '动物' | '美食' | '物品' | '地点自然' | '动作职业'
  hint: string
  aliases?: string[]
}

export const DRAWING_WORDS: DrawingWord[] = [
  { word: '小猫', category: '动物', hint: '会发出轻柔叫声的宠物', aliases: ['猫', '猫咪'] },
  { word: '小狗', category: '动物', hint: '热情忠诚的宠物', aliases: ['狗', '狗狗'] },
  { word: '长颈鹿', category: '动物', hint: '脖子很长的草食动物' },
  { word: '熊猫', category: '动物', hint: '黑白相间，喜欢竹子' },
  { word: '企鹅', category: '动物', hint: '穿着黑白礼服的极地居民' },
  { word: '兔子', category: '动物', hint: '长耳朵，擅长跳跃', aliases: ['小兔子'] },
  { word: '章鱼', category: '动物', hint: '生活在海里，有许多触手' },
  { word: '海豚', category: '动物', hint: '聪明友善的海洋动物' },
  { word: '袋鼠', category: '动物', hint: '腹部有育儿袋，跳得很远' },
  { word: '松鼠', category: '动物', hint: '大尾巴，喜欢收集坚果' },
  { word: '猫头鹰', category: '动物', hint: '夜晚活动，眼睛很大' },
  { word: '金鱼', category: '动物', hint: '常养在鱼缸里的小动物' },

  { word: '冰淇淋', category: '美食', hint: '夏天很受欢迎的冰凉甜品' },
  { word: '汉堡包', category: '美食', hint: '面包中夹着肉和蔬菜', aliases: ['汉堡'] },
  { word: '小笼包', category: '美食', hint: '皮薄多汁的中式点心' },
  { word: '西瓜', category: '美食', hint: '绿色外皮、红色果肉的水果' },
  { word: '草莓', category: '美食', hint: '红色心形的小浆果' },
  { word: '火锅', category: '美食', hint: '大家围着一口热锅吃' },
  { word: '爆米花', category: '美食', hint: '看电影时常见的零食' },
  { word: '蛋糕', category: '美食', hint: '庆祝生日时常会出现' },
  { word: '胡萝卜', category: '美食', hint: '橙色的根茎蔬菜' },
  { word: '荷包蛋', category: '美食', hint: '完整蛋黄在中间的煎蛋', aliases: ['煎蛋'] },
  { word: '棉花糖', category: '美食', hint: '蓬松柔软，像云朵一样' },
  { word: '奶茶', category: '美食', hint: '茶和牛奶组合的饮料' },

  { word: '雨伞', category: '物品', hint: '下雨天用来遮挡雨水', aliases: ['伞'] },
  { word: '书包', category: '物品', hint: '学生背着它去上学' },
  { word: '闹钟', category: '物品', hint: '每天按时叫人起床' },
  { word: '吉他', category: '物品', hint: '有六根弦的常见乐器' },
  { word: '相机', category: '物品', hint: '用来记录画面的设备', aliases: ['照相机'] },
  { word: '眼镜', category: '物品', hint: '戴在眼前帮助看清东西' },
  { word: '拖鞋', category: '物品', hint: '在家里穿的轻便鞋子' },
  { word: '牙刷', category: '物品', hint: '每天早晚清洁牙齿' },
  { word: '风筝', category: '物品', hint: '借助风在天空中飞' },
  { word: '台灯', category: '物品', hint: '放在桌面上照亮书本' },
  { word: '钥匙', category: '物品', hint: '可以打开特定的锁' },
  { word: '热气球', category: '物品', hint: '靠热空气升上天空' },

  { word: '小房子', category: '地点自然', hint: '温暖的住处', aliases: ['房子', '家'] },
  { word: '游乐园', category: '地点自然', hint: '有许多娱乐设施的地方' },
  { word: '彩虹', category: '地点自然', hint: '雨后天空中的七彩弧线' },
  { word: '火山', category: '地点自然', hint: '可能喷出岩浆的山' },
  { word: '沙滩', category: '地点自然', hint: '海边铺满细沙的地方' },
  { word: '雪人', category: '地点自然', hint: '冬天用积雪堆成的人形' },
  { word: '月亮', category: '地点自然', hint: '夜空中明亮的天体' },
  { word: '瀑布', category: '地点自然', hint: '从高处奔流而下的水' },
  { word: '电影院', category: '地点自然', hint: '在大银幕前观看影片' },
  { word: '图书馆', category: '地点自然', hint: '安静阅读和借书的地方' },
  { word: '摩天轮', category: '地点自然', hint: '缓慢转动的巨大圆形设施' },
  { word: '樱花树', category: '地点自然', hint: '春天会开粉色花朵的树' },

  { word: '跳绳', category: '动作职业', hint: '让绳子反复从脚下经过' },
  { word: '踢足球', category: '动作职业', hint: '用脚把球送进球门' },
  { word: '睡觉', category: '动作职业', hint: '闭上眼睛休息很久' },
  { word: '洗澡', category: '动作职业', hint: '用水清洁身体' },
  { word: '拍照', category: '动作职业', hint: '按下快门记录画面' },
  { word: '做饭', category: '动作职业', hint: '把食材变成一顿餐食' },
  { word: '医生', category: '动作职业', hint: '在医院帮助病人康复' },
  { word: '消防员', category: '动作职业', hint: '穿防护服扑灭火灾' },
  { word: '宇航员', category: '动作职业', hint: '乘坐飞船探索太空' },
  { word: '画家', category: '动作职业', hint: '用画笔创作图像的人' },
  { word: '钓鱼', category: '动作职业', hint: '拿着鱼竿等待鱼儿上钩' },
  { word: '骑自行车', category: '动作职业', hint: '踩踏板让两个轮子前进' },
]

export function wordForRoom(roomCode: string, round: number) {
  let hash = round * 97
  for (const character of roomCode) hash = (hash * 31 + character.charCodeAt(0)) >>> 0
  return DRAWING_WORDS[hash % DRAWING_WORDS.length]
}
