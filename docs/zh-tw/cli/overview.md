# 正式 CLI

`cle` 用於執行一次性的引擎工作。目前的參數為 `-version`、`-check-config <file>`、`-config <file>`、`-launch <request.json>` 和 `--json`。Go flag 解析器接受單橫線或雙橫線形式。執行 `cle -h` 可查看用法。

正式 CLI 不包含互動式除錯終端機。舊命令 `cle debug` 只會顯示遷移提示，請改用獨立程式 `cle-debug`。例如 `cle account list`、`cle java list` 並不是正式 CLI 命令。

請參閱[命令參考](commands.md)和[JSON 輸出](json-output.md)。
