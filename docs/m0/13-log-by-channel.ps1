# M0 ⑬：GET /api/log/ 按渠道过滤、字段、分页
. "$PSScriptRoot\_lib.ps1"
$ts = [int][DateTimeOffset]::UtcNow.AddHours(-1).ToUnixTimeSeconds()
"--- channel=2 start_timestamp=$ts"
$r = Api GET "/api/log/?p=1&page_size=10&type=2&channel=2&start_timestamp=$ts"
"total=$($r.data.total) page=$($r.data.page) page_size=$($r.data.page_size) items=$($r.data.items.Count) channels_in_items=" + (($r.data.items | % channel | Sort-Object -Unique) -join ',')
"字段: " + (($r.data.items[0].PSObject.Properties.Name) -join ',')
"--- channel=1"
$r1 = Api GET "/api/log/?p=1&page_size=5&type=2&channel=1&start_timestamp=$ts"
"total=$($r1.data.total) items=$($r1.data.items.Count) channels_in_items=" + (($r1.data.items | % channel | Sort-Object -Unique) -join ',')
"--- 不带 type（含 type=5 错误日志等）"
$ra = Api GET "/api/log/?p=1&page_size=200&channel=1&start_timestamp=$ts"
"types: " + (($ra.data.items | Group-Object type | % { "type$($_.Name)=$($_.Count)" }) -join ' ')
"--- 分页：page_size=3, p=1,2；排序 id desc"
(Api GET "/api/log/?p=1&page_size=3&type=2&channel=1").data.items | % id
"--"
(Api GET "/api/log/?p=2&page_size=3&type=2&channel=1").data.items | % id
"--- page_size 上限试探：100 / 101 / 1000 / 100000"
foreach ($ps in 100,101,1000,100000) { $x = Api GET "/api/log/?p=1&page_size=$ps&type=2"; "page_size=$ps -> 返回 page_size=$($x.data.page_size) items=$($x.data.items.Count) total=$($x.data.total)" }
"--- 越界页 p=999"
$x = Api GET "/api/log/?p=999&page_size=10&type=2"; "items=$($x.data.items.Count) total=$($x.data.total)"
"--- 不存在渠道 channel=999"
$x = Api GET "/api/log/?channel=999"; "total=$($x.data.total) items=$($x.data.items.Count)"
"--- 无 min_id/id 过滤参数（源码 controller/log.go:13-26 未读取）：传 min_id=9999 看是否被忽略"
$x = Api GET "/api/log/?min_id=9999&page_size=2&type=2"; "total=$($x.data.total) items=$($x.data.items.Count)"
