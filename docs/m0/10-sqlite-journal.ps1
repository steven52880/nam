# M0 ⑩：容器内 SQLite 的 journal_mode。只读打开宿主侧文件（不改库）。
. "$PSScriptRoot\_lib.ps1"
$sh = Join-Path $Env 'pragma.sh'
wsl -d Ubuntu -- bash "/mnt/f/Mess/newapi-configurator/temp/m0-env/pragma.sh"
# 文件头第 18/19 字节：01 01 = 非 WAL（delete/rollback）；02 02 = WAL
# 代码侧：common/database.go:15 SQLitePath = "one-api.db?_busy_timeout=30000"；全仓库无 journal_mode/PRAGMA 设置
