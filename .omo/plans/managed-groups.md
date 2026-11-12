# 托管分组（managed groups）实施计划

## 背景与现状

- NewAPI 渠道的 `group` 是逗号分隔列表（如 `claude,share,default`），主人会在 NewAPI 里手改。
- **托管分组** = 所有 `store.Vendor.GroupName` 的集合。nam 只能增删改托管分组，非托管分组原样保留、顺序不变。
- 现状（已逐条核对）：
    - `channels/service.go:250-256` saveChanges：改厂商且没传 `fields.group` 时，把 group **整串覆盖**为厂商分组。清空厂商不动 group。
    - `channels/validate.go:99` 用整串 `r.Group` 查 `ix.byGroup`；`:129` 要求 `r.Group == v.GroupName`（整串相等）。
    - `channels/snapshot.go:28` snapOf 存整串 group；`:66` diffSnap 整串比较；`:203` pruneFields 整串比较。
    - `channels/drift.go:45,84` 两处调 diffSnap；`:124` OverwriteDrift 把 `applied.Group` 整串写回草稿。
    - `channels/apply.go:77-80` planItem 用 `overlay` + `toPatch`；`:222-235` applyUpdate 直接把草稿 group 写 NewAPI。
    - `channels/create.go:185-188` 批量新建只写 `v.GroupName`，**保持不变**（已确认）。
    - `vendors/vendors.go:184-192` Update：有渠道在用时拒绝改 name / group_name。
    - `store/store.go:45` `SetMaxOpenConns(1)`：**事务内不能再用 `s.St.DB` 或调 `s.load`**，否则死锁。
    - `applied_hash` 只写不读（grep 确认），改 snapshot 内容不影响任何比较。
    - 前端**没有独立的分组列**：分组只出现在 `EditableCells.tsx:83` 厂商单元格的 title；`constants.ts:42` `vendor: ['vendor','group']` 负责高亮。
- 包依赖：`vendors` 只 import `resp`、`store`、gorm；`channels` 不 import `vendors`。所以 **`channels` → `vendors` 可以加，不会成环**。`api` 已同时持有 `Channels` 和 `Vendors`（`api/api.go:24,26`）。

## 公共工具 `nam/internal/channels/groups.go`（新文件）

- `splitGroups(s string) []string`  
  按 `,` 切，每项 `TrimSpace`，丢空串，去重保留首次出现。`""`、`" , "` → `nil`。
- `joinGroups(gs []string) string`：`strings.Join(gs, ",")`，nil → `""`。
- `normGroups(s string) string`：`joinGroups(splitGroups(s))`。
- `containsGroup(gs []string, g string) bool`。
- `(ix *vendorIndex) isManaged(g string) bool`：`_, ok := ix.byGroup[g]`。
- `(st *state) groupPred(drop []string) func(string) bool`  
  返回 `g` 属于托管分组 **或** 在 `drop` 里。`drop` 见下文 `GroupDrop`。
- `managedSubset(gs []string, isManaged func(string) bool) []string`：保序过滤。
- `sameGroupSet(a, b []string) bool`：忽略顺序的集合相等（漂移比较用）。
- `mergeManaged(live, draft []string, isManaged func(string) bool) []string`  
  非托管取自 live，托管取自 draft，位置尽量保留：
    - `want := managedSubset(draft)`。
    - 遍历 live：非托管 → 原样输出；托管且在 want 中 → 原位输出并标记已用；托管但不在 want 中 → 记一个**空位**（不输出）。
    - want 里未输出的按顺序**依次填入空位**（按空位出现顺序插在原位置），空位用完后追加到末尾；多余空位丢弃。
    - 结果再去重。
    - 例：live `a,old,b`、draft `new`、old 在 drop 中 → `a,new,b`（改名保持位置）。
- `swapVendorGroup(gs []string, oldG, newG string, isManaged func(string) bool) []string`（改厂商用）
    - `newG == ""`（清空厂商）：只删除 `oldG`，其余不动。
    - 否则：插入位置 = `oldG` 的位置；没有 `oldG` 时取第一个托管分组的位置；都没有就追加末尾。删除 `oldG` 和**所有其他托管分组**，在该位置放 `newG`，去重。
    - 理由：绑定厂商的渠道不允许含其他厂商分组（校验会拒），而分组列改为只读后用户无法在 nam 里修正，否则会卡死。
- `renameGroup(gs []string, oldG, newG string) []string`：原位替换，去重（`old,new` → `new`）。

## 各文件改动

### `channels/types.go`

- `draftBody` 新增 `GroupDrop []string \`json:"group_drop,omitempty"\``：apply 时即使已不是托管分组也要从 live 删除的分组（只由厂商分组改名产生）。旧草稿没有该字段，反序列化为 nil，兼容。
- `VendorUpdate` 结构（给 API 用）：`store.Vendor`（内嵌，JSON 扁平，兼容旧响应）+ `GroupDrafts int \`json:"group_drafts"\`` + `RenamedDrafts int \`json:"renamed_drafts"\``。

### `channels/service.go`

- `row()`（L96 后）：若 `isLive && body.Fields.Group != nil`，`c.Group = joinGroups(mergeManaged(splitGroups(live.Group), splitGroups(*body.Fields.Group), st.groupPred(body.GroupDrop)))`。create 草稿仍直接用草稿整串。
- `saveChanges` 每条 change（替换 L249-256）：
    - 先 `before := st.row(id)`（本条改动前的有效厂商与分组，包含同批前序改动）。
    - `pred := st.groupPred(body.GroupDrop)`。
    - **`fields.group` 语义：只取托管部分**：若 `ch.Fields.Group != nil`，替换为 `joinGroups(mergeManaged(splitGroups(before.Group), splitGroups(*ch.Fields.Group), pred))`。选这个而不拒绝，因为 OverwriteDrift 内部仍要传 group；apply 时本来也只合并托管部分，语义一致。
    - 改厂商：`ch.Ext.Vendor != nil && *ch.Ext.Vendor != before.Vendor` 时：
        - 新厂商非空且未知 → 不动 group，留给 validateRows 报错（同现状）。
        - `oldG` = `before.Vendor` 对应的 GroupName（未知/空则 `""`），`newG` = 新厂商 GroupName（清空则 `""`）。
        - `base` = 上一步算出的 `fields.Group`（若有），否则 `before.Group`；`fields.Group = joinGroups(swapVendorGroup(splitGroups(base), oldG, newG, pred))`。
        - 新旧都为 `""` 时不动。
    - `pruneFields` 调用改为 `pruneFields(live, body.Fields, pred)`；若 group 被剪掉，同时 `body.GroupDrop = nil`。
- L282 空草稿判断不变（GroupDrop 只在 Fields.Group 非空时有意义）。

### `channels/snapshot.go`

- `snapOf` **不变**：applied_snap 继续存整串 group。旧行与新行格式一致，无需迁移。
- `diffSnap(applied, now snapshot, isManaged func(string) bool)`：group 比较改为 `!sameGroupSet(managedSubset(split(applied.Group)), managedSubset(split(now.Group)))`。用**当前**厂商索引判定托管，因此手改非托管分组、旧快照里的非托管分组都不算漂移。
- `pruneFields(c, f, isManaged)`：group 剪枝条件改为 `joinGroups(mergeManaged(split(c.Group), split(*f.Group), isManaged)) == normGroups(c.Group)`。

### `channels/drift.go`

- `CheckDrift` L45、`Drift` L84：传 `st.vendors.isManaged`。
- `Drift` 详情里 group 的 Old/New 显示 `joinGroups(managedSubset(...))`，让用户看到真正漂移的部分。
- `AcceptDrift`：不变（存远端整串；比较时只看托管部分 = "接受远端托管分组"）。
- `OverwriteDrift`：不变，`applied.Group` 走 saveChanges 的"只取托管部分"语义，只恢复托管分组，保留远端非托管分组。
- `Adopt`：不变。

### `channels/apply.go`

- `planItem` 默认分支：`c := overlay(live, body.Fields)` 后套用与 `row()` 相同的合并（抽成 `st.effective(live, body) ports.Channel` 供 row / planItem 共用）。`Request.Body` 的 patch：`Group` 设为合并后的值；若等于 `normGroups(live.Group)` 则置 nil。`fieldChanges` 自然显示合并值。
- `applyUpdate`：若 `body.Fields.Group != nil`，**apply 时重新 `s.Channels.Get`**，`merged := mergeManaged(split(fresh.Group), split(*body.Fields.Group), st.groupPred(body.GroupDrop))`；等于 `normGroups(fresh.Group)` 时 `p.Group = nil`，否则 `p.Group = &merged`。若 patch 全部字段为 nil 且 `p.Key == nil`，跳过 `Update` 直接 finish。可与 `keyToWrite` 共用一次 Get（把 live 传进去），非必须。
- `applyCreate`、`toCreate`：不变。

### `channels/validate.go`

- 第一轮循环（L99-107）：`gs := splitGroups(r.Group)`，对每个托管 `g` 记 `modelGroups[m][g] = true`（不再整串查 byGroup）。
- 厂商检查（L124-132）：
    - 未知厂商 → 同现状。
    - `!containsGroup(gs, v.GroupName)` → `add("group", "渠道分组必须包含厂商 X 的分组 G")`。
    - 任一托管 `g != v.GroupName` → `add("group", "渠道分组不能包含其他厂商 Y 的分组 g")`（`ix.byGroup[g].Name`）。
    - 非托管分组一律忽略；无厂商的渠道不做分组检查。
- 更新 L81-87 注释。

### `channels/vendor_rename.go`（新文件，`channels` import `vendors`）

- `func (s *Service) UpdateVendor(ctx, id uint, in vendors.Input) (VendorUpdate, error)`：
    - `s.mu.Lock()`（会改草稿，必须与其他草稿操作串行）。
    - **事务外**：`st, err := s.load(ctx)`（含 NewAPI live 列表、全部草稿、ext、旧厂商索引）。List 失败 → 返回 `resp.Upstream` 错误，什么都不改。
    - 事务内（只用 `tx`）：
        - `old, cur, err := vendors.UpdateTx(tx, id, in)`（含 404、冲突检查）。
        - 名称变了：`tx.Model(&store.ChannelExt{}).Where("vendor = ?", old.Name).Update("vendor", cur.Name)`；遍历 `st.drafts`，`body.Ext.Vendor == old.Name` 的改为新名，记入 `touched`，`RenamedDrafts++`。
        - 分组变了：对 `st.rows()` 的每个 id（live + create 草稿，**含无 ext 的渠道**）：
            - 有 delete 草稿 → 跳过。
            - `gs := splitGroups(st.row(id).Group)`（已合并草稿），不含 `old.GroupName` → 跳过。
            - `body.Fields.Group = joinGroups(renameGroup(gs, old.GroupName, cur.GroupName))`；live 渠道追加 `old.GroupName` 到 `body.GroupDrop`（去重）。
            - 无草稿则新建 `DraftUpdate` 草稿；记入 `touched`，`GroupDrafts++`。
        - `touched` 里的每个草稿 `LastError=""`、`UpdatedAt=now` 后 `upsertDraft(tx, &d)`。
    - 不调用 validateRows，不写 NewAPI。
- 注意：草稿 diff 里的 Ext.Vendor 和 Fields.Group 在同一 body 上改，一个渠道同时改名两项只算一次 upsert。

### `vendors/vendors.go`

- 删除 `Update`，新增 `func UpdateTx(tx *gorm.DB, id uint, in Input) (old, cur store.Vendor, err error)`：normalize → `tx.First` → `checkConflicts`（"新 group_name 不能被其他厂商占用"保留）→ apply → Save。**去掉 L184-192 的在用拒绝**。
- 更新 L170 注释。`Delete` 的在用检查保留。

### `api/vendors.go`

- `updateVendor`（L54）改调 `s.Channels.UpdateVendor`，响应返回 `VendorUpdate`。审计摘要：`修改厂商 X（生成 N 个分组改名草稿）`。

### 前端

- `api/types-channels.ts`：新增 `interface VendorUpdateResult extends Vendor { group_drafts: number; renamed_drafts: number }`；`endpoints.ts:130` `updateVendor` 返回类型改为它。
- `settings/VendorSection.tsx:49-53`：`onSuccess(res)`，编辑模式且 `group_drafts > 0` 时 `toast.success(\`厂商已保存，已为 ${n} 个渠道生成分组改名草稿，请到渠道页应用\`)`，否则原提示；同时 invalidate `['channels']`（名称改动也影响渠道行）。
- `channels/columns.tsx`：在厂商列后加只读列 `readOnly('group', '分组', (r) => r.group, '托管分组随厂商变化，其他分组请在 NewAPI 中维护', { width: 160 })`。
- `channels/constants.ts:42`：新增 `group: ['group']`；`vendor` 保留 `['vendor']`。
- `EditableCells.tsx:83`：title 改为 `分组：${row.group}（托管分组随厂商变化，其他分组请在 NewAPI 中维护）`。

## 兼容与迁移

- applied_snap 格式不变（整串），旧行无需迁移；比较时才投影到托管子集。
- 旧草稿：Fields.Group 是整串，按新语义只取托管部分合并 live；无 GroupDrop 字段 → nil。
- 已绑定厂商、但 live 分组被旧逻辑覆盖成单一厂商分组的渠道：新逻辑下合法，无需处理。
- 不加表、不加列，`AutoMigrate` 无变化。

## 测试

**`channels/groups_test.go`（新，表驱动）**

- `TestSplitGroups`：`""`、`" a , ,b,a "` → `[a b]`、单项。
- `TestMergeManaged`：保留非托管；draft 增托管（追加）；draft 删托管；改名经 drop 保位置（`a,old,b`→`a,new,b`）；live 新增手改非托管被保留；结果去重。
- `TestSwapVendorGroup`：旧组原位替换；无旧组追加；清空只删旧组；其他托管组被移除；新组已存在去重。
- `TestRenameGroup`：原位、`old,new`→`new`、不含 old 不变。

**`channels/service_test.go` / `m35_test.go`**

- 改 `TestVendorChangeSetsGroup`：期望 `default,anthropic`。
- 改 `TestAdoptIdenticalKeyDoesNotWriteKey` L127：期望 `default,anthropic`。
- `TestVendorClearRemovesOnlyVendorGroup`：`default,anthropic` 清厂商 → `default`。
- `TestValidationManagedGroups`：`default,anthropic` 通过；`default` 缺组报错；`anthropic,gemini` 报错；两个渠道同模型分属两个托管组报错，非托管组同模型不报。
- `TestGroupFieldOnlyManagedPart`：对 `default` 无厂商渠道提交 `group: "foo,gemini"` → 行为 `default,gemini`。
- `TestApplyMergesLiveGroups`：存改厂商草稿后在 fake 里把 live 改成 `default,vip`；Plan 的 group 变更与 patch 都是 `default,vip,anthropic`；Apply 后 fake 同值。
- `TestDriftIgnoresUnmanagedGroups`：apply 后 live 加 `vip` → 无漂移；换成 `gemini` → 漂移 `group`；OverwriteDrift + Apply 后恢复 anthropic 且保留 `vip`。
- `TestDriftLegacySnapshot`：手写 AppliedSnap 整串 `default,anthropic`，live `anthropic,default,vip` → 无漂移。

**`channels/vendor_rename_test.go`（新）**

- `TestVendorGroupRenameCreatesDrafts`：绑定渠道 `default,anthropic`、无 ext 渠道 `anthropic,vip`、create 草稿、delete 草稿、已有 priority 草稿的渠道；改 group anthropic→claude：`GroupDrafts` 计数正确，delete 草稿不动，priority 保留，fake 未被写；Apply 后 live 为 `default,claude` / `claude,vip`。
- `TestVendorNameRenameUpdatesExtAndDrafts`：ext.vendor 与草稿 ext.vendor 都改名，NewAPI 未写。
- `TestVendorRenameConflict`：group 改成 gemini → 409，数据不变。
- `TestVendorRenameListFails`：包一层 List 返回错误的 ChannelStore → 返回错误，厂商未改。

**`vendors/vendors_test.go`**

- `TestInUseVendorIsProtected` L82-84 改为：在用时通过 `UpdateTx` 改名成功；Delete 仍 409。

## 验证命令

- `go build ./...`、`go vet ./...`（workdir `nam`，timeout 120s）。
- `go test ./internal/channels/... ./internal/vendors/... ./internal/api/... ./core/...`（timeout 300s）。
- `npm run build`（workdir `web`，含 `tsc --noEmit`，timeout 180s）。
- 核心规则自检：core 不 import QuantumNous；没有新 goroutine；NewAPI 写入只经 `s.Channels`（已审计包装）。

## 开放风险

- **改名后用户丢弃草稿**：live 仍是旧组，旧组已不托管，绑定渠道下次被改动时会报"必须包含分组"。可接受，提示文案已说明需应用。
- **分组列原本不存在**：计划新增只读列，若主人不想多一列，可只改厂商单元格 title。
- `swapVendorGroup` 会移除渠道上的其他厂商分组（超出"只换旧组"的字面规则），理由见上；需主人确认。
- 改名在锁内先 List NewAPI 再开事务；期间 NewAPI 被手改的分组不会进入草稿，但 apply 时会重新读 live 合并，不会丢手改。
- 托管判定依赖当前厂商表：删除厂商后其分组变为非托管，旧绑定渠道上的该组将被永久保留（Delete 本就要求无渠道在用）。
- 漂移比较忽略托管分组顺序；NewAPI 对分组顺序无语义，风险低。
