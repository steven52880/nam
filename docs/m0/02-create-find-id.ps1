# M0 ②：POST /api/channel/ 不返回 ID；用唯一名称搜索拿 ID
. "$PSScriptRoot\_lib.ps1"
$uniq = 'nam-m0-' + [guid]::NewGuid().ToString('N').Substring(0,8)
$ch = @{name=$uniq; type=58; key='upstream-key-B'; base_url=$MockIn; models='test-model'; group='default'; priority=1; weight=1; status=1; test_model='test-model'}
"--- 缺 mode 外壳（直接发 channel 对象）"
Api POST '/api/channel/' $ch | ConvertTo-Json -Compress
"--- 正确形态 {mode:single, channel:{...}}"
$c = NewChannel $ch
$c | ConvertTo-Json -Compress
"--- 搜索"
$f = Api GET "/api/channel/search?keyword=$uniq&id_sort=true"
"total=$($f.data.total) exact=$(@($f.data.items | ? name -eq $uniq).Count) id=$(($f.data.items | ? name -eq $uniq).id)"
"--- 搜索响应顶层字段: " + (($f.data.PSObject.Properties | % Name) -join ',')
"--- 清理"
$id = ($f.data.items | ? name -eq $uniq).id
Api DELETE "/api/channel/$id" | ConvertTo-Json -Compress
