# CLI 命令參考

以下參數由 `cmd/cle` 解析，不使用位置子命令。

## `cle -version`

用途：顯示程式版本。沒有必填參數。一般輸出為版本文字；加上 `--json` 時回傳 JSON 回應。

```text
$ cle -version
ChesLaunchEngine dev
```

## `cle -check-config <file>`

用途：讀取並驗證 CLE 設定 JSON；`<file>` 為必填。成功時顯示 `configuration is valid`；失敗時回傳非零結束碼。JSON 模式會回傳結構化錯誤。

## `cle -config <file>`

用途：指定本次執行使用的 CLE 設定。可省略，省略時使用內建預設值；可與 `-launch` 或 `-check-config` 組合。單獨指定設定不會啟動服務。

```text
cle -config cle.json -launch launch.json
```

設定不存在或無效時會回傳非零結束碼；JSON 模式下錯誤回應寫入 stdout，診斷資訊寫入 stderr。

## `cle -launch <request.json>`

用途：讀取啟動要求並啟動 Minecraft；要求檔為必填，CLE 設定可透過 `-config <cle.json>` 指定。程式會啟動並管理 Java 處理程序，直到其結束。Ctrl+C 會取消啟動工作，或要求正在執行的遊戲處理程序停止。

```powershell
cle -config .\cle.json -launch .\examples\launch\vanilla.json
```

成功時顯示工作 ID 和 PID；JSON 模式會回傳最終工作物件。失敗時回傳非零結束碼及錯誤。

## `cle --json`

為支援的一次性命令輸出單一 JSON 回應；stdout 不會混入一般記錄，診斷資訊寫入 stderr。成功結束碼為 0，失敗則為非零。

```text
cle --json -version
cle --json -check-config cle.json
cle --json -launch request.json
```

## `cle debug`

僅用於顯示遷移提示，不會初始化或匯入除錯終端機。請改用 `cle-debug`。
