# flx Usage Guide

鏈枃妗ｆ寜鈥滄蹇?-> API 鍒嗙被 -> 杈圭晫琛屼负 -> 瀹炴垬寤鸿鈥濈殑椤哄簭鏁寸悊 `flx` 鐨勫畬鏁翠娇鐢ㄦ柟寮忋€?
濡傛灉浣犲彧鏄涓€娆′笂鎵嬶紝寤鸿鍏堢湅 [quickstart.md](./quickstart.md)銆?
鏈枃妗ｄ腑鐨勫畬鏁村鍏ヨ矾寰勯粯璁や娇鐢?`github.com/ezra-sullivan/flx`锛屼唬鐮侀噷鐨勫寘鍚嶄粛鐒舵槸 `flx`銆?
## 1. 蹇冩櫤妯″瀷

`flx` 鐨勬牳蹇冩槸锛?
```go
type Stream[T any]
```

瀹冧唬琛ㄤ竴鏉″熀浜?channel 鐨勫紓姝ュ鐞嗙閬擄紝鑰屼笉鏄櫘閫氬垏鐗囧寘瑁呫€?
浣犲彲浠ユ妸瀹冪悊瑙ｄ负锛?
- 涓婃父鎸佺画浜у嚭 `T`
- 涓棿鎿嶄綔鎸佺画娑堣垂涓婃父骞朵骇鍑烘柊鐨勭粨鏋?- 缁堢粨鎿嶄綔璐熻矗鏀跺彛

杩欏甫鏉ヤ袱涓洿鎺ュ奖鍝嶏細

- 鏌愪簺涓棿鎿嶄綔鍦ㄨ皟鐢ㄦ椂灏变細鍒涘缓 goroutine 鎴?channel
- 鏈€濂芥€绘槸璋冪敤涓€涓粓缁撴搷浣滐紝鎶婃祦鏄惧紡娑堣垂瀹屾垚

## 2. API 鎬讳綋璁捐

`flx` 閲囩敤涓ょ被 API锛?
### 鍚岀被鍨嬫搷浣滀繚鐣欎负鏂规硶

杩欎簺鎿嶄綔涓嶄細寮曞叆鏂扮殑绫诲瀷鍙傛暟锛?
- `Concat`
- `Filter`
- `Buffer`
- `Sort`
- `Reverse`
- `Head`
- `Tail`
- `Skip`
- 鍚勭被缁堢粨鏂规硶

### 璺ㄧ被鍨嬫搷浣滀娇鐢ㄥ寘绾ф硾鍨嬪嚱鏁?
杩欎簺鎿嶄綔浼氭妸 `Stream[T]` 鍙樻垚 `Stream[U]`锛屽洜姝ゅ啓鎴愬寘绾у嚱鏁帮細

- `Map`
- `MapErr`
- `FlatMap`
- `FlatMapErr`
- `MapContext`
- `MapContextErr`
- `FlatMapContext`
- `FlatMapContextErr`
- `Reduce`

杩欐槸涓轰簡閫傚簲 Go 1.26 浠嶇劧涓嶆敮鎸侀澶栫被鍨嬪弬鏁版柟娉曠殑鐜板疄绾︽潫銆?
## 3. 鏋勯€?Stream

### `Values`

閫傚悎灏戦噺鍥哄畾鍊硷細

```go
s := flx.Values(1, 2, 3)
```

### `From`

閫傚悎鐢ㄤ唬鐮佷富鍔ㄧ敓浜у厓绱狅細

```go
s := flx.From(func(out chan<- int) {
	for i := range 10 {
		out <- i
	}
})
```

绾︽潫锛?
- 浣犲彧璐熻矗鍐欏叆锛屼笉闇€瑕佸叧闂?channel
- `flx` 浼氬湪鐢熸垚鍑芥暟杩斿洖鍚庡叧闂唴閮?channel
- 濡傛灉鐢熶骇鍑芥暟 panic锛宲anic 浼氳璁板綍鍒?stream 閿欒鐘舵€侊紱`*Err` 缁堢粨鎿嶄綔浼氳繑鍥炶閿欒锛岄粯璁ょ粓缁撴搷浣滃湪 fail-fast 涓嬩細 panic

### `FromChan`

閫傚悎鎺ョ鐜版湁鍙 channel锛?
```go
s := flx.FromChan(ch)
```

绾︽潫锛?
- 璋冪敤鏂瑰繀椤讳繚璇佷笂娓告渶缁堝叧闂?channel

### `Concat`

鎶婂涓悓绫诲瀷 stream 鍚堝苟鍒颁竴涓緭鍑?stream锛?
```go
merged := flx.Concat(
	flx.Values(1, 2),
	flx.Values(3, 4),
)
```

涔熷彲浠ョ敤鏂规硶褰㈠紡锛?
```go
merged := flx.Values(1, 2).Concat(flx.Values(3, 4))
```

琛屼负璇存槑锛?
- 浼氬悎骞朵笂娓搁敊璇姸鎬侊紝鑰屼笉鏄涪鎺夊畠浠?- 涓嶅悓涓婃父涔嬮棿鐨勮緭鍑虹浉瀵归『搴忎笉淇濊瘉
- 姣忎釜涓婃父鍐呴儴浠嶄繚鎸佸悇鑷殑杈撳嚭椤哄簭

## 4. 鍚岀被鍨嬩腑闂存搷浣?
### `Filter`

鍙繚鐣欐弧瓒虫潯浠剁殑鍏冪礌锛?
```go
s := flx.Values(1, 2, 3, 4).Filter(func(v int) bool {
	return v%2 == 0
})
```

### `Buffer`

璋冩暣褰撳墠闃舵杈撳嚭 channel 鐨勭紦鍐插尯澶у皬锛?
```go
s := upstream.Buffer(128)
```

### `Sort`

鎶婂綋鍓?stream 鍏ㄩ噺鏀堕泦鍚庢帓搴忥紝鍐嶉噸鏂拌緭鍑猴細

```go
s := flx.Values(3, 1, 2).Sort(func(a, b int) bool {
	return a < b
})
```

娉ㄦ剰锛?
- 杩欐槸涓€涓叏閲忔搷浣滐紝涓嶆槸娴佸紡鎺掑簭
- `less` 闇€瑕佹弧瓒崇ǔ瀹氫笖鑷唇鐨勬瘮杈冨叧绯?
### `Reverse`

鍏ㄩ噺鏀堕泦鍚庡弽杞『搴忥細

```go
s := flx.Values(1, 2, 3).Reverse()
```

### `Head`

淇濈暀鍓?`n` 涓厓绱狅細

```go
s := flx.Values(1, 2, 3, 4).Head(2)
```

绾︽潫锛?
- `n < 1` 浼?panic

琛ュ厖璇存槑锛?
- `Head` 鍦ㄦ嬁鍒板墠 `n` 涓厓绱犲悗锛屼粛浼氱户缁?drain 涓婃父鍐嶅叧闂嚜宸辩殑杈撳嚭
- 杩欐牱鍙互淇濊瘉涓婃父 goroutine 鏀舵暃锛屽苟璁╁悗缁?`*Err` 缁堢粨鎿嶄綔鐪嬪埌鏅氬埌鐨?worker error

### `Tail`

淇濈暀鏈€鍚?`n` 涓厓绱狅細

```go
s := flx.Values(1, 2, 3, 4).Tail(2)
```

绾︽潫锛?
- `n < 1` 浼?panic

### `Skip`

璺宠繃鍓?`n` 涓厓绱狅細

```go
s := flx.Values(1, 2, 3, 4).Skip(2)
```

绾︽潫锛?
- `n < 0` 浼?panic

## 5. 璺ㄧ被鍨嬪彉鎹?
### `Map`

1:1 鏄犲皠锛?
```go
out := flx.Map(flx.Values(1, 2, 3), func(v int) string {
	return strconv.Itoa(v)
})
```

### `MapErr`

1:1 鏄犲皠锛屽厑璁歌繑鍥為敊璇細

```go
out := flx.MapErr(flx.Values("1", "2", "x"), strconv.Atoi)
items, err := out.CollectErr()
```

### `FlatMap`

1:N 鏄犲皠锛岀敱 worker 涓诲姩鍚戜笅娓稿啓鍏ワ細

```go
out := flx.FlatMap(flx.Values(1, 2, 3), func(v int, pipe chan<- int) {
	pipe <- v
	pipe <- v * 10
})
```

### `FlatMapErr`

鍜?`FlatMap` 涓€鏍凤紝浣嗗厑璁歌繑鍥為敊璇細

```go
out := flx.FlatMapErr(in, func(v Item, pipe chan<- Result) error {
	if err := validate(v); err != nil {
		return err
	}

	pipe <- convert(v)
	return nil
})
```

### `MapContext` / `FlatMapContext`

鍦?worker 涓樉寮忎娇鐢?`context.Context`锛?
```go
ctx := context.Background()

out := flx.MapContext(ctx, flx.Values(1, 2, 3), func(ctx context.Context, v int) int {
	select {
	case <-ctx.Done():
		return 0
	default:
		return v * 10
	}
})
```

閫傜敤鍦烘櫙锛?
- worker 闇€瑕佸搷搴斿彇娑堟垨瓒呮椂
- 闇€瑕侀厤鍚堝己鍒跺姩鎬佸苟鍙?- 鍚戜笅娓稿彂閫佹椂闇€瑕?`SendContext`

### `MapContextErr` / `FlatMapContextErr`

鍦ㄦ樉寮?context 鍩虹涓婂悓鏃惰繑鍥為敊璇細

```go
out := flx.MapContextErr(ctx, in, func(ctx context.Context, v string) (int, error) {
	return strconv.Atoi(v)
})
```

### `DistinctBy`

鎸?key 鍘婚噸锛屼繚鐣欑涓€娆″嚭鐜扮殑鍏冪礌锛?
```go
out := flx.DistinctBy(users, func(u User) string {
	return u.Email
})
```

杈圭晫璇存槑锛?
- `DistinctBy` 浼氱淮鎶ゅ綋鍓嶆祦涓墍鏈夊凡瑙?key
- 瀹冮€傚悎鏈夋槑纭竟鐣岀殑鎵瑰鐞嗘祦
- 濡傛灉鍞竴 key 鏁版寔缁闀匡紝鍐呭瓨浼氶殢涔嬫寔缁闀?- 瀵规棤鐣屾祦鎴栬秴楂樺熀鏁版祦锛屼笉寤鸿鐩存帴浣跨敤鍏ㄥ眬 `DistinctBy`

### `GroupBy`

鎸?key 鍒嗙粍锛岃緭鍑?`Stream[[]T]`锛?
```go
out := flx.GroupBy(users, func(u User) string {
	return u.Team
})
```

琛屼负璇存槑锛?
- 姣忕粍鍐呴儴淇濇寔鍘熷杈撳叆椤哄簭
- 缁勪笌缁勪箣闂存寜 key 绗竴娆″嚭鐜扮殑椤哄簭杈撳嚭
- `GroupBy` 浼氬湪杈撳嚭鍓嶆秷璐瑰苟缂撳瓨鏁存潯杈撳叆娴?- 瀹冧笉閫傚悎瓒呭ぇ鏁版嵁闆嗘垨鏃犵晫娴?- 濡傛灉浣犻渶瑕佽竟鐣屽寲鍐呭瓨鐨勫垎缁勶紝浼樺厛鑰冭檻绐楀彛鍖栧垎缁勬垨鑷畾涔夎仛鍚堥€昏緫

### `DistinctByCount` / `GroupByCount`

鎸夎緭鍏ユ暟閲忓仛 tumbling window锛?
```go
out := flx.DistinctByCount(users, 1000, func(u User) string {
	return u.Email
})

groups := flx.GroupByCount(users, 1000, func(u User) string {
	return u.Team
})
```

琛屼负璇存槑锛?- 姣?`n` 涓緭鍏ュ厓绱犳瀯鎴愪竴涓獥鍙?- `DistinctByCount` 鍙湪绐楀彛鍐呭幓閲嶏紱璺ㄧ獥鍙ｇ浉鍚?key 浼氶噸鏂板嚭鐜?- `GroupByCount` 鍙湪绐楀彛鍐呭垎缁勶紱杈撳嚭绫诲瀷鏄?`Stream[flx.Group[K, T]]`
- 鎸夐噺绐楀彛姣斿叏灞€ `DistinctBy` / `GroupBy` 鏇存帴杩戠‖鍐呭瓨杈圭晫

### `DistinctByWindow` / `GroupByWindow`

鎸?processing-time 鍋?tumbling window锛?
```go
ctx := context.Background()

out := flx.DistinctByWindow(ctx, users, 5*time.Second, func(u User) string {
	return u.Email
})

groups := flx.GroupByWindow(ctx, users, 5*time.Second, func(u User) string {
	return u.Team
})
```

琛屼负璇存槑锛?- 绐楀彛鍦ㄦ敹鍒扮涓€涓厓绱犳椂鍚姩锛岀┖绐楀彛涓嶈緭鍑?- `DistinctByWindow` 鍙湪褰撳墠鏃堕棿绐楀彛鍐呭幓閲?- `GroupByWindow` 浼氬湪绐楀彛缁撴潫鎴栦笂娓稿叧闂椂杈撳嚭褰撳墠绐楀彛鍐呯殑鎵€鏈?group
- `ctx.Done()` 鍚庝細鍋滄澶勭悊锛屽苟灏藉揩鍥炴敹涓婃父
- 鎸夋椂绐楀彛鎻愪緵鐨勬槸杞竟鐣岋紱濡傛灉浣犻渶瑕佹洿绋冲畾鐨勫唴瀛樹笂鐣岋紝浼樺厛浣跨敤鎸夐噺绐楀彛

### `Chunk`

鎸夊浐瀹氬ぇ灏忓垎鍧楋細

```go
out := flx.Chunk(flx.Values(1, 2, 3, 4, 5), 2)
```

杈撳嚭椤哄簭绛変环浜庯細

- `[1, 2]`
- `[3, 4]`
- `[5]`

绾︽潫锛?
- `n < 1` 浼?panic

### `Reduce`

鎶婃暣涓?source channel 浜ょ粰鑷畾涔夊綊绾﹀嚱鏁帮細

```go
sum, err := flx.Reduce(flx.Values(1, 2, 3), func(ch <-chan int) (int, error) {
	var sum int
	for v := range ch {
		sum += v
	}
	return sum, nil
})
```

閫傚悎锛?
- 鍋氳嚜瀹氫箟鑱氬悎
- 闇€瑕佹帶鍒舵秷璐归€昏緫

娉ㄦ剰锛?
- `Reduce` 杩斿洖鐨勯敊璇細鍜?stream 鍐呴儴閿欒鍚堝苟
- 濡傛灉 reducer 鎻愬墠杩斿洖锛宍flx` 浠嶄細鑷姩 drain 鍓╀綑杈撳叆锛屼繚璇佷笂娓?goroutine 鑳芥甯告敹鏁?
## 6. 缁堢粨涓庢煡璇?
### `Done` / `DoneErr`

浠呮秷璐瑰畬娴侊紝涓嶅叧蹇冪粨鏋滐細

```go
out.Done()
err := out.DoneErr()
```

### `ForEach` / `ForEachErr`

閫愪釜娑堣垂锛?
```go
out.ForEach(func(v int) {
	fmt.Println(v)
})
```

### `ForAll` / `ForAllErr`

鎶婂簳灞?channel 鏆撮湶缁欒嚜瀹氫箟娑堣垂閫昏緫锛?
```go
out.ForAll(func(ch <-chan int) {
	for v := range ch {
		fmt.Println(v)
	}
})
```

### `Parallel` / `ParallelErr`

鎶婂綋鍓?stream 褰撲綔浠诲姟婧愶紝骞跺彂鎵ц鍓綔鐢細

```go
err := flx.Values("a", "b", "c").ParallelErr(func(v string) error {
	return process(v)
}, control.WithWorkers(4))
```

### `Count` / `CountErr`

缁熻鍏冪礌鏁伴噺锛?
```go
n := out.Count()
n, err := out.CountErr()
```

### `Collect` / `CollectErr`

鏀堕泦鎴愬垏鐗囷細

```go
items := out.Collect()
items, err := out.CollectErr()
```

### `First` / `FirstErr`

鑾峰彇绗竴涓厓绱狅細

```go
v, ok := out.First()
v, ok, err := out.FirstErr()
```

绌烘祦鏃讹細

- `ok == false`

琛ュ厖璇存槑锛?
- `First` 浠嶇劧灞炰簬闈?`*Err` 缁堢粨鎿嶄綔
- 濡傛灉涓婃父宸茬粡璁板綍浜?fail-fast 閿欒锛宍First` 浼氬儚 `Done` / `Collect` 涓€鏍?panic
- `First` 鍦ㄦ嬁鍒伴涓厓绱犲悗锛屼粛浼氱瓑寰呬笂娓?drain 瀹屾垚锛屽啀鍐冲畾鏄惁鏆撮湶鏅氬埌鐨?fail-fast 閿欒
- `FirstErr` 鐜板湪鏄湡姝ｇ煭璺細鎷垮埌棣栦釜鍏冪礌鍚庝細绔嬪嵆杩斿洖锛屽苟鍦ㄥ悗鍙扮户缁?drain 涓婃父
- `FirstErr` 杩斿洖鐨勬槸褰撳墠鏃跺埢鐨勯敊璇揩鐓э紱杩斿洖鍚庢墠鍙戠敓鐨?fail-fast error 涓嶄繚璇佸寘鍚湪 `err` 涓?- 濡傛灉浣犻渶瑕佺ǔ瀹氱殑鏈€缁堥敊璇竟鐣岋紝浼樺厛浣跨敤 `CollectErr`锛屾垨鑰呯敤 `Head(1).CollectErr()` 鏄惧紡淇濈暀棣栦釜鍏冪礌

### `Last` / `LastErr`

鑾峰彇鏈€鍚庝竴涓厓绱狅細

```go
v, ok := out.Last()
v, ok, err := out.LastErr()
```

### `AllMatch` / `AnyMatch` / `NoneMatch`

鍋氬竷灏旂煭璺垽鏂細

```go
all := out.AllMatch(func(v int) bool { return v > 0 })
any := out.AnyMatch(func(v int) bool { return v == 0 })
none := out.NoneMatch(func(v int) bool { return v < 0 })
```

濡傛灉浣犻渶瑕佹樉寮忛敊璇鐞嗭紝浼樺厛浣跨敤锛?
- `AllMatchErr`
- `AnyMatchErr`
- `NoneMatchErr`

琛ュ厖璇存槑锛?
- 杩欎簺闈?`*Err` 鐭矾缁堢粨鎿嶄綔鍚屾牱閬靛惊 fail-fast 璇箟
- 濡傛灉涓婃父宸茬粡璁板綍 fail-fast 閿欒锛屽畠浠笉浼氶潤榛樺悶閿欙紝鑰屾槸浼?panic
- 鍛戒腑鐭矾鏉′欢鍚庯紝瀹冧滑浠嶄細绛夊緟涓婃父 drain 瀹屾垚锛岄伩鍏嶆櫄鍒扮殑 fail-fast 閿欒涓㈠け
- 瀵瑰簲鐨?`*Err` 鐗堟湰鐜板湪鏄湡姝ｇ煭璺細鍛戒腑鏉′欢鍚庣珛鍗宠繑鍥烇紝骞跺湪鍚庡彴 drain 涓婃父
- 杩欎簺 `*Err` 鐗堟湰杩斿洖鐨勬槸褰撳墠鏃跺埢鐨勯敊璇揩鐓э紱杩斿洖鍚庢墠鍙戠敓鐨?fail-fast error 涓嶄繚璇佸寘鍚湪 `err` 涓?- 濡傛灉浣犻渶瑕佺ǔ瀹氱殑鏈€缁堥敊璇竟鐣岋紝浼樺厛浣跨敤 `DoneErr` / `CollectErr` 绛夊畬鏁存秷璐瑰瀷缁堢粨鎿嶄綔

### `Max` / `Min`

鎸?`less` 瑙勫垯閫夊嚭鏈€澶?鏈€灏忓€硷細

```go
best, ok := out.Max(func(a, b int) bool { return a < b })
least, ok := out.Min(func(a, b int) bool { return a < b })
```

濡傛灉闇€瑕侀敊璇繑鍥烇紝浣跨敤锛?
- `MaxErr`
- `MinErr`

### `Err`

璇诲彇褰撳墠 stream 宸茶褰曠殑閿欒鐘舵€侊細

```go
err := out.Err()
```

鏇存帹鑽愮殑鍋氭硶浠嶇劧鏄洿鎺ュ湪缁堢粨鐐逛娇鐢?`*Err`銆?
## 7. 骞跺彂閫夐」

骞跺彂 `Option` 鍙綔鐢ㄤ簬褰撳墠鎿嶄綔锛屼笉浼氳嚜鍔ㄥ悜涓嬫父浼犳挱銆?
### `WithWorkers`

鍥哄畾骞跺彂鏁帮細

```go
out := flx.Map(in, worker, control.WithWorkers(8))
```

璇存槑锛?
- `workers <= 0` 浼氳鍐呴儴鎻愬崌鍒版渶灏忓€?`1`

### `WithUnlimitedWorkers`

姣忎釜鍏冪礌涓€涓?worker锛?
```go
out := flx.Map(in, worker, control.WithUnlimitedWorkers())
```

閫傚悎锛?
- 浠诲姟閲忓皬
- 鎴栧崟涓换鍔℃瀬鎱€佷笖浣犳槑纭帴鍙楅珮 goroutine 鏁?
### `WithDynamicWorkers`

浼橀泤鍔ㄦ€佸苟鍙戯紝缂╁鏃朵笉鎵撴柇宸茶繍琛?worker锛?
```go
ctrl := control.NewConcurrencyController(4)
out := flx.Map(in, worker, control.WithDynamicWorkers(ctrl))
ctrl.SetWorkers(8)
ctrl.SetWorkers(2)
```

### `WithForcedDynamicWorkers`

寮哄埗鍔ㄦ€佸苟鍙戯紝缂╁鏃跺彇娑堝浣?worker锛?
```go
ctrl := control.NewConcurrencyController(4)
out := flx.MapContext(ctx, in, worker, control.WithForcedDynamicWorkers(ctrl))
```

閲嶈绾︽潫锛?
- 鍙兘鐢ㄤ簬 `MapContext*` / `FlatMapContext*`
- 濡傛灉鐢ㄤ簬闈?context 鍙樻崲锛屼細 panic

### `WithInterruptibleWorkers`

鍏煎鍒悕锛?
```go
out := flx.MapContext(ctx, in, worker, control.WithInterruptibleWorkers(ctrl))
```

鏂颁唬鐮佸缓璁粺涓€鏀规垚 `WithForcedDynamicWorkers`銆?
## 8. 閿欒妯″瀷

### 榛樿绛栫暐锛歚ErrorStrategyFailFast`

worker 杩斿洖閿欒鎴?panic 鏃讹細

- 璁板綍閿欒
- 鍙栨秷褰撳墠鎿嶄綔
- 鍦ㄧ粓缁撻樁娈垫毚闇查敊璇?
### `ErrorStrategyCollect`

缁х画鎵ц鎵€鏈?worker锛屾渶鍚庢妸閿欒鑱氬悎璧锋潵锛?
```go
out := flx.MapErr(
	in,
	worker,
	control.WithErrorStrategy(control.ErrorStrategyCollect),
)
```

### `ErrorStrategyContinue`

缁х画鎵ц锛屼絾涓嶆妸閿欒鍐欏叆 stream 鐘舵€侊細

```go
out := flx.MapErr(
	in,
	worker,
	control.WithErrorStrategy(control.ErrorStrategyContinue),
)
```

`ErrorStrategyLogAndContinue` 浠嶄繚鐣欎负鍏煎鍒悕锛屼絾涓嶅啀鍐呭缓杈撳嚭鏃ュ織銆?

### `WorkerError`

worker 鏄惧紡杩斿洖閿欒鎴?panic 閮戒細缁熶竴鍖呰涓?`WorkerError`锛?
```go
var workerErr *control.WorkerError

if errors.As(err, &workerErr) {
	...
}
```

### 瀹為檯寤鸿

- 涓氬姟浠ｇ爜閲岋紝濡傛灉浣犻渶瑕佺ǔ瀹氱殑鏈€缁堥敊璇竟鐣岋紝浼樺厛浣跨敤瀹屾暣娑堣垂鍨?`*Err` API锛屼緥濡?`DoneErr` / `CollectErr`
- `FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 鐜板湪鏄綆寤惰繜鐭矾鏌ヨ锛岃繑鍥炵殑鏄綋鍓嶉敊璇揩鐓ц€屼笉鏄渶缁堥敊璇泦鍚?- 鍙湁浣犳槑纭帴鍙椻€滃嚭閿欏嵆 panic鈥濇椂锛屽啀浣跨敤闈?`*Err` 缁堢粨鎿嶄綔
- 鍋氭壒澶勭悊缁熻鏃讹紝`ErrorStrategyCollect` 寰€寰€姣?`fail-fast` 鏇村悎閫?- `flx` 榛樿鎶婇敊璇缓妯′负 stream 鐘舵€侊紱濡傛灉浣犵‘瀹炶鎶娾€滄瘡鏉℃暟鎹嚜宸辩殑澶辫触缁撴灉鈥濈户缁線涓嬫父浼狅紝璇疯嚜宸卞畾涔変笟鍔＄粨鏋勪綋锛屽苟鎶婂畠褰撴櫘閫氭暟鎹鐞?
### item 绾х粨鏋滄ā寮?
濡傛灉浣犱粠 `fx` 杩佺Щ杩囨潵锛屼箣鍓嶄範鎯妸 `error` 濉炲洖姣忎釜 item锛屽彲浠ョ户缁繖鏍峰啓锛屼絾瑕佹妸瀹冭涓轰笟鍔℃暟鎹紝鑰屼笉鏄?`flx` 鐨勫畼鏂归敊璇ā鍨嬶細

```go
type ItemResult[T any] struct {
	Value T
	Err   error
}

out := flx.Map(flx.Values("1", "x", "3"), func(v string) ItemResult[int] {
	n, err := strconv.Atoi(v)
	return ItemResult[int]{Value: n, Err: err}
})

results := out.Collect()
```

閫傜敤鍦烘櫙锛?
- 浣犺淇濈暀姣忔潯璁板綍鐨勬垚鍔?澶辫触缁撴灉锛屾渶鍚庣粺涓€缁熻銆佽惤搴撴垨褰掓。
- 浣犳帴鍙楀崟鏉″け璐ヤ笉缁堟娴佹按绾?
涓嶉€傜敤鍦烘櫙锛?
- 浣犲笇鏈?worker error銆乸anic銆乼imeout銆乧ontext cancel 璧扮粺涓€鐨?stream 閿欒杈圭晫
- 浣犻渶瑕?`DoneErr` / `CollectErr` 鏆撮湶鏈€缁堥敊璇?
## 9. context 涓庡彇娑?
### 鏄惧紡浼犲叆 context

`flx` 涓嶅啀閫氳繃 option 娉ㄥ叆鐖?context锛岃€屾槸鐩存帴鍐欐垚 API 鍙傛暟锛?
```go
out := flx.MapContext(ctx, in, worker)
```

杩欐牱鍋氱殑濂藉鏄細

- context 鏉ユ簮鏄庣‘
- API 璇箟鏇寸ǔ瀹?- 涓嶉渶瑕?`WithParentContext`

### `SendContext`

褰?worker 鍚戜笅娓稿啓鍏ユ椂锛屽鏋滀綘甯屾湜鍦ㄥ彇娑堝悗灏藉揩鍋滄鍙戦€侊紝浣跨敤锛?
```go
ok := flx.SendContext(ctx, pipe, item)
```

杩斿洖鍊硷細

- `true`锛氬彂閫佹垚鍔?- `false`锛氬彂閫佸墠宸茶鍙栨秷

琛ュ厖璇存槑锛?
- `MapContext` / `MapContextErr` 鐨勫崟缁撴灉鍙戦€佸凡缁忓唴寤哄彇娑堟劅鐭?- 浣犺嚜宸卞湪 `FlatMapContext*` 鎴栧叾瀹冭嚜瀹氫箟 worker 涓悜涓嬫父鍙戦€佸€兼椂锛屼粛鐒跺簲璇ヤ紭鍏堜娇鐢?`SendContext`

### 缂╁鍙栨秷鍘熷洜

寮哄埗鍔ㄦ€佸苟鍙戜笅锛屽鏋?worker 鍥犵缉瀹硅鍙栨秷锛屽彲浠ュ垽鏂細

```go
if errors.Is(context.Cause(ctx), control.ErrWorkerLimitReduced) {
	...
}
```

## 10. 鐙珛骞跺彂宸ュ叿

闄や簡 stream API锛宍flx` 杩樻彁渚涗簡鍖呯骇骞跺彂鎵ц宸ュ叿銆?
### `Parallel`

骞跺彂鎵ц澶氫釜鏃犺繑鍥為敊璇殑鍑芥暟锛屽け璐ユ椂 panic锛?
```go
flx.Parallel(task1, task2, task3)
```

### `ParallelWithErrorStrategy`

涓哄苟鍙戞墽琛岃缃敊璇瓥鐣ワ細

```go
err := flx.ParallelWithErrorStrategy(
	control.ErrorStrategyCollect,
	task1,
	task2,
)
```

### `ParallelErr`

骞跺彂鎵ц澶氫釜杩斿洖閿欒鐨勫嚱鏁帮紝骞惰仛鍚堥敊璇細

```go
err := flx.ParallelErr(task1, task2, task3)
```

## 11. retry

### `DoWithRetry`

瀵规櫘閫氬嚱鏁板仛閲嶈瘯锛?
```go
err := flx.DoWithRetry(
	call,
	flx.WithRetry(3),
	flx.WithInterval(200*time.Millisecond),
)
```

### `DoWithRetryCtx`

瀵规樉寮忔帴鏀?context 鐨勫嚱鏁板仛閲嶈瘯锛?
```go
err := flx.DoWithRetryCtx(
	context.Background(),
	func(ctx context.Context, attempt int) error {
		return callWithContext(ctx, attempt)
	},
	flx.WithRetry(5),
)
```

### Retry 閫夐」

- `WithRetry(n)`锛氶噸璇曟鏁?- `WithInterval(d)`锛氭瘡娆￠噸璇曢棿闅?- `WithTimeout(d)`锛氭暣涓噸璇曡繃绋嬬殑鎬昏秴鏃?- `WithAttemptTimeout(d)`锛氬崟娆″皾璇曡秴鏃讹紝浠?`DoWithRetryCtx` 鍙敤
- `WithIgnoreErrors(errs)`锛氬懡涓繖浜涢敊璇椂鐩存帴瑙嗕负鎴愬姛

娉ㄦ剰锛?
- `WithAttemptTimeout` 鐢ㄥ湪 `DoWithRetry` 涓婁細 panic
- 鍚勭璐熸椂闂存垨闈炴硶娆℃暟浼?panic

## 12. timeout

### `DoWithTimeout`

缁欐櫘閫氬嚱鏁板寘涓€灞傝秴鏃讹細

```go
err := flx.DoWithTimeout(call, 5*time.Second)
```

### `DoWithTimeoutCtx`

缁欐帴鏀?context 鐨勫嚱鏁板寘涓€灞傝秴鏃讹細

```go
err := flx.DoWithTimeoutCtx(func(ctx context.Context) error {
	return callWithContext(ctx)
}, 5*time.Second)
```

### `WithContext`

缁?timeout 鎸囧畾鐖?context锛?
```go
err := flx.DoWithTimeoutCtx(
	func(ctx context.Context) error { return callWithContext(ctx) },
	5*time.Second,
	flx.WithContext(parent),
)
```

### `WithTimeoutLatePanicObserver`

濡傛灉 timeout 宸茬粡杩斿洖锛屼絾鍥炶皟鍚庣画鍙堝彂鐢?panic锛屽彲浠ラ€氳繃 observer 鎷跨粺涓€鍙娴嬩簨浠讹細

```go
err := flx.DoWithTimeoutCtx(
	func(ctx context.Context) error {
		return callWithContext(ctx)
	},
	5*time.Second,
	flx.WithTimeoutLatePanicObserver(func(event flx.TimeoutLatePanicEvent) {
		logger.Printf("late timeout panic: %v", event.Panic)
	}),
)
```

### 甯歌閿欒

- `ErrCanceled`
- `ErrTimeout`
- `ErrNilContext`
- `ErrNegativeTimeout`

## 13. 鏈€浣冲疄璺?
### 1. 榛樿浼樺厛绋冲畾閿欒杈圭晫鐨勭粓缁撴搷浣?
濡傛灉浣犻渶瑕佹渶缁堥敊璇粨鏋滐紝浼樺厛浣跨敤浼氬畬鏁存秷璐?source 鐨勭粓缁撴搷浣滐紝渚嬪 `DoneErr` / `CollectErr`銆?
`FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 鏇撮€傚悎浣庡欢杩熺煭璺煡璇紝鑰屼笉鏄嬁鏈€缁堥敊璇泦鍚堛€?
### 2. worker 鍚戜笅娓稿啓鍏ユ椂浼樺厛 `SendContext`

灏ゅ叾鏄湪 `FlatMapContext*` 鍜屽己鍒跺姩鎬佸苟鍙戜笅銆?
`MapContext*` 鐨勫崟缁撴灉鍙戦€佸凡缁忓唴寤哄彇娑堟劅鐭ワ紝浣嗚嚜瀹氫箟澶氭鍙戦€佸満鏅粛鐒跺簲鏄惧紡浣跨敤 `SendContext`銆?
### 3. `WithUnlimitedWorkers` 瑕佽皑鎱?
瀹冨緢鏂逛究锛屼絾涔熸渶瀹规槗鏀惧ぇ goroutine 鏁伴噺鍜屽唴瀛樺帇鍔涖€?
### 4. 鍏ㄩ噺鎴栫疮璁″唴瀛樻搷浣滆娉ㄦ剰鎴愭湰

浠ヤ笅鎿嶄綔浼氭妸褰撳墠娴佹暣浣撴垨鏄捐憲绱鍒板唴瀛樹腑锛?
- `Sort`
- `Reverse`
- `Collect`
- `DistinctBy`
- `GroupBy`

琛ュ厖璇存槑锛?
- `Tail(n)` 鍙繚鐣欐渶鍚?`n` 涓厓绱?- `Chunk(n)` 鍙繚鐣欏綋鍓?chunk锛屼笉浼氫竴娆℃€х紦瀛樻暣涓祦
- `DistinctBy` 鐨勬垚鏈殢鍞竴 key 鏁板闀?- `GroupBy` 鐨勬垚鏈殢鏁存潯杈撳叆娴佸闀?- `DistinctByCount` / `GroupByCount` 鎻愪緵鎸夐噺 tumbling window锛屾洿鎺ヨ繎纭竟鐣?- `DistinctByWindow` / `GroupByWindow` 鎻愪緵鎸夋椂 tumbling window锛屽睘浜庤蒋杈圭晫

### 5. 闇€瑕佺ǔ瀹氶敊璇涓烘椂锛岀敤 `DoneErr` / `CollectErr`

杩欐牱姣斾緷璧?panic 鏇撮€傚悎绾夸笂鏈嶅姟鍜屽崟鍏冩祴璇曘€?
## 14. 甯歌杩佺Щ鎻愰啋

浠?`fx` 杩佺Щ鍒?`flx` 鏃讹紝鏈€瀹规槗韪╃殑鍑犱釜鐐规槸锛?
- `Just` 鏀瑰悕涓?`Values`
- `Range` 鏀瑰悕涓?`FromChan`
- `stream.Map(...)` 鏀逛负 `flx.Map(stream, ...)`
- `stream.Walk(...)` 鏀逛负 `flx.FlatMap(stream, ...)`
- `WalkCtx*` 鏀逛负鏄惧紡 `ctx` 鐨?`MapContext*` / `FlatMapContext*`
- `First` / `Last` / `Max` / `Min` 鐜板湪閮借繑鍥?`ok`
- 濡傛灉浣犱互鍓嶅湪娴侀噷浼?`struct{ Value T; Err error }` 杩欑被缁撴灉瀵硅薄锛宍flx` 閲岀户缁嚜瀹氫箟鍗冲彲锛屼絾涓嶈鎶婂畠鍜?`MapErr` / `CollectErr` 瑙嗕负鍚屼竴灞傞敊璇涔?
濡傞渶浠?`fx` 杩佺Щ锛岃缁撳悎 README 鍜?`quickstart.md` 涓殑 API 瀵圭収璇存槑閫愭鏇挎崲銆?
## 15. 实战示例

如果你想看一条包含多个 stage、并且每个 stage 使用不同 worker 策略的图片处理流水线，可以参考这个完整示例：

- [图片处理流水线示例](./examples/imgproc-pipeline.md)

这个示例展示了：

- Picsum 分页列图
- 固定 worker 下载
- 动态 worker 做缩放
- 固定 worker 加水印并保存
- 推荐的 stage 函数组合写法
- 对照的原生 `flx` API 逐段拼接写法

主线示例的 list / download 走真实 Picsum，retry 示例单独保持本地可控。

## 16. Pipeline Coordinator

如果你希望根据 stage backlog、link backlog 和资源压力去调节动态 worker，而不是在业务代码里手动 `SetWorkers(...)`，就使用 `pipeline/coordinator`。

如果你想系统看 `pipeline/coordinator` 和 `pipeline/observe` 的分工、最小接法、`Snapshot()` / `Tick()` 关系，以及 resource observer 的使用方式，优先看单独文档：

- [coordinator 与 observe 使用说明](./coordinator-observe.md)

### 核心接法

典型接法有四步：

- 创建 `PipelineCoordinator`
- 在 stage 上挂 `coordinator.WithCoordinator(...)`
- 给 stage 标名字 `coordinator.WithStageName(...)`
- 只给需要被调节的动态 stage 配 `coordinator.WithStageBudget(...)`

当前适用边界：

- 一个 `PipelineCoordinator` 实例按“一次 pipeline run”使用；它会在整个实例生命周期内保留最近一次看到的 stage/link/control 状态
- `Links` 视图当前按 `fromStage` 聚合，只保留每个 stage 的一条 outbound link snapshot
- 因此目前更适合线性 pipeline；如果后面要支持 fan-out / DAG，需要先扩展 link 表示模型

示例：

```go
pipelineCoordinator := coordinator.NewPipelineCoordinator(
	coordinator.PipelineCoordinatorPolicy{
		ScaleUpStep:         1,
		ScaleDownStep:       1,
		ScaleUpCooldown:     500 * time.Millisecond,
		ScaleDownCooldown:   750 * time.Millisecond,
		ScaleUpHysteresis:   1,
		ScaleDownHysteresis: 2,
	},
	coordinator.WithResourceObserver(myResourceObserver),
)

images := flx.Stage(
	ctx,
	listedImages,
	download,
	control.WithWorkers(4),
	coordinator.WithCoordinator(pipelineCoordinator),
	coordinator.WithStageName("download"),
).Through(
	ctx,
	resize,
	control.WithForcedDynamicWorkers(resizeController),
	coordinator.WithCoordinator(pipelineCoordinator),
	coordinator.WithStageName("resize"),
	coordinator.WithStageBudget(coordinator.StageBudget{MinWorkers: 2, MaxWorkers: 8}),
)
```

### `Snapshot()` 和 `Tick()`

`PipelineCoordinator` 当前是显式控制模型：

- `Snapshot()` 返回当前 pipeline 观测视图
- `Tick()` 做一次策略判断并尝试调整 worker

当前 `Snapshot()` 包含三部分：

- `Stages`
- `Links`
- `Resources`

也就是说，你现在可以同时看到：

- stage 内部 backlog
- stage 间 link backlog
- 外部资源压力样本

资源采样契约补充：

- `Snapshot()` 和 `Tick()` 都会各自独立轮询 resource observer
- 同一个 coordinator 实例会串行化这些轮询，避免同一个 observer 被 `Snapshot()` / `Tick()` 并发重入
- 因此，紧挨着打印出来的 snapshot，不保证就是下一次 `Tick()` 真正用于决策的那一份 resource sample
- 如果同一个 observer 被复用到多个 coordinator 实例，observer 仍然需要自己保证线程安全

### `PipelineCoordinatorPolicy` 字段语义

- `ScaleUpStep`
  每次扩容最多增加多少 worker。
- `ScaleDownStep`
  每次缩容最多减少多少 worker。
- `ScaleUpCooldown`
  同一 stage 两次扩容之间的最小时间间隔。
- `ScaleDownCooldown`
  同一 stage 两次缩容之间的最小时间间隔。
- `ScaleUpHysteresis`
  扩容信号需要连续满足多少个 `Tick()` 才会真正扩容。
- `ScaleDownHysteresis`
  缩容信号需要连续满足多少个 `Tick()` 才会真正缩容。

默认值是保守兼容的：

- `ScaleUpStep <= 0` 按 `1` 处理
- `ScaleDownStep <= 0` 按 `1` 处理
- `ScaleUpCooldown < 0` 按 `0` 处理
- `ScaleDownCooldown < 0` 按 `0` 处理
- `ScaleUpHysteresis <= 0` 按 `1` 处理
- `ScaleDownHysteresis <= 0` 按 `1` 处理

### 当前策略重点

当前 `Tick()` 的优先级大致是：

1. `budget_min`
2. 资源 `critical` 下对空闲 stage 的 shrink bias
3. `incoming_link_backlog`
4. `stage_backlog`
5. `idle_shrink`

当前语义重点：

- 下游 link backlog 优先于上游缩容
- `warning` 级资源压力会 brake scale-up
- `critical` 级资源压力只会对满足 `activeWorkers < currentWorkers` 的空闲 stage 施加 shrink bias
- `budget_min` 是硬纠偏，不受 cooldown / hysteresis 限制
- cooldown / hysteresis 是按 stage、按方向分别生效的

### 适用边界

当前 coordinator 只会调整：

- 挂了 `WithCoordinator(...)`
- 使用了动态 controller
- 同时配置了 `WithStageBudget(...)`

的 stage。

这意味着：

- 固定 worker stage 仍然可以被观测
- 但不会被 coordinator 改 worker 数

### Resource Observer

`WithResourceObserver(...)` 可以接任何资源压力采样源，只要它能返回 `observe.ResourceSnapshot`。

最常见的接法包括：

- 内存
- 网络
- CPU
- 外部队列长度
- 下游服务限流状态

实现约束建议：

- observer 应尽量快速返回，因为它运行在调用 `Snapshot()` / `Tick()` 的 goroutine 上
- 同一个 coordinator 内部不会并发重入同一个 observer
- 但跨多个 coordinator 复用同一个 observer 时，仍然要由 observer 自己处理同步

当前内置示例使用的是基于 `runtime.MemStats` 的 memory observer，完整可运行接法见 [图片处理 Coordinator 示例](./examples/imgproc-coordinator.md)。
