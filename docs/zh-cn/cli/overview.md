# 正式 CLI

`cle` 执行一次性的引擎操作。当前参数为 `-version`、`-check-config <file>`、`-config <file>`、`-launch <request.json>` 和 `--json`。Go flag 解析器接受单横线或双横线形式。查看用法可执行 `cle -h`。

正式 CLI 不包含交互式调试台。旧命令 `cle debug` 只显示迁移提示，请改用独立程序 `cle-debug`。例如 `cle account list`、`cle java list` 并不是正式 CLI 命令。

参阅[命令参考](commands.md)和[JSON 输出](json-output.md)。
