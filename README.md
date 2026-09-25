
# mackerel-plugin-jsonl

JSON Lines 形式のログからメトリックを生成し、Mackerelに投稿するためのカスタムプラグインです。

## インストール方法

mkrを使うと楽です

```
mkr plugin install --upgrade monitoring-forge/mackerel-plugin-jsonl
```

## 基本的な使い方

```
./mackerel-plugin-jsonl \
	--key-name count \
	--json-key foo.bar \
	--aggregator count \
	--log-file /path/to/your.log \
	--prefix custom.jsonl
```

複数のキーや集計方法を指定する場合は、`--key-name`, `--json-key`, `--aggregator` を同じ数だけ指定してください。

## mackerel-agent.conf 設定例

```ini
[plugin.metrics.jsonl]
command = "/path/to/mackerel-plugin-jsonl --key-name count --json-key foo.bar --aggregator count --log-file /var/log/app.log --prefix custom.jsonl"
```


## aggregatorの種類と説明

`--aggregator` には以下の種類を指定できます。

- `count` : 対象となるキーが存在する行をカウントします
	- 例: `json.total.count        80000.000000    1760281381`
- `group_by` : 指定したキーの値ごとに件数を集計します。
	- 例: `json.status.2xx 24769.733333    1760281381`
- `group_by_with_percentage` : group_byの各値ごとの割合（%）も出力します。
	- 例: `json.status_percentage.5xx      23.139667       1760281381`
- `percentile` : 指定した数値キーの平均値とパーセンタイル（mean, p90, p95, p99）を出力します。
	- 例: `json.latency.p95        0.030000        1760281381`

複数のaggregatorを同時に指定することで、複数のメトリクスを一度に出力できます。

### percentile の出力項目をカスタマイズする

`percentile(...)` の引数に、出力したい統計値・パーセンタイルをカンマ区切りで指定できます。計算には sampdo を使用します。

```sh
./mackerel-plugin-jsonl \
  --key-name latency \
  --json-key reqtime \
  --aggregator 'percentile("max","min","mean","50","70",90,99.9)' \
  --log-file /path/to/your.log \
  --prefix json
```

- 統計値は `max`（最大値）、`min`（最小値）、`mean`（平均値）を指定できます。
- パーセンタイルは `0` 以上 `100` 以下の数値で、小数も指定できます。
- 引数は引用符あり・なしのどちらでも指定できます。
- 統計値の出力名はそのまま `max`、`min`、`mean` です。数値は先頭に `p` を付け、小数点を `_` に変換します（`99.9` → `p99_9`）。

上記の例では、`json.latency.max`、`json.latency.min`、`json.latency.mean`、`json.latency.p50`、`json.latency.p70`、`json.latency.p90`、`json.latency.p99_9` のみを指定順に出力します。

従来の `--aggregator percentile` は引き続き `mean`、`p90`、`p95`、`p99` を出力します。`percentile()` のような空の指定、未対応の統計名、範囲外の数値はエラーになります。対象データがない場合は出力しません。

---

## json-keyの変換関数

`--json-key` にはパイプ（`|`）区切りで変換関数を指定できます。

例: `foo.bar|tolower|replace('a','b')|trimspace`

利用可能な関数:

- `tolower` : 小文字化
- `toupper` : 大文字化
- `trimspace` : 前後の空白を除去
- `replace('pattern','repl')` : 正規表現で置換
- `have('key1','key2','key3')` : 初期状態のキーのリスト。集計した値がない時に `0` で結果を生成できます

例:
```
--json-key "user.name|trimspace|tolower"
--json-key "message|replace('error','warn')"
```

## mackerel-plugin-jsonlによるメトリクス例(1)

一般的なJSON形式のWebサーバのアクセスログからメトリクスを生成する

```
% /path/to/mackerel-plugin-jsonl --prefix json --log-file json.log \
  -k total.count -j time -a count \
  -k status -j 'status|replace("^(?:([1235])\d{2}|(4)(?:[0-8]\d|9[0-8]))$","${1}${2}xx")|have("2xx","3xx","4xx","499","5xx")' -a group_by_with_percentage \
  -k latency -j reqtime -a percentile
json.total.count        4800000.000000  1760339314
json.status.3xx 684256.000000   1760339314
json.status.4xx 1370792.000000  1760339314
json.status.499 344392.000000   1760339314
json.status.5xx 1031020.000000  1760339314
json.status.2xx 1369540.000000  1760339314
json.status_percentage.2xx      28.532083       1760339314
json.status_percentage.3xx      14.255333       1760339314
json.status_percentage.4xx      28.558167       1760339314
json.status_percentage.499      7.174833        1760339314
json.status_percentage.5xx      21.479583       1760339314
json.latency.mean       0.030000        1760339314
json.latency.p99        0.030000        1760339314
json.latency.p90        0.030000        1760339314
json.latency.p95        0.030000        1760339314
```

## mackerel-plugin-jsonlによるメトリクス例(2)

dnstap (https://github.com/dmachard/DNS-collector) のJSONログからメトリクス生成

```
mackerel-plugin-jsonl \
  --prefix dnstap \
  -l /var/log/dnstap/query.log \
  -k network.proto \
  -j 'network.protocol|tolower|have(tcp,udp)' \
  -a group_by \
  -k dns.qtype \
  -j 'dns.qtype|tolower|have(a,aaaa)' \
  -a group_by_with_percentage \
  -k dns.latency \
  -j dnstap.latency \
  -a percentile \
  -k dns.rcode \
  -j 'dns.rcode|tolower|have(noerror,servfail,nxdomain,refused)' \
  -a group_by \
  --per-second
dnstap.network.proto.udp	25.818182	1760582504
dnstap.network.proto.tcp	0.000000	1760582504
dnstap.dns.qtype.a	19.303030	1760582504
dnstap.dns.qtype.aaaa	3.666667	1760582504
dnstap.dns.qtype.srv	0.030303	1760582504
dnstap.dns.qtype.https	2.757576	1760582504
dnstap.dns.qtype.mx	0.030303	1760582504
dnstap.dns.qtype.txt	0.030303	1760582504
dnstap.dns.qtype_percentage.a	74.765258	1760582504
dnstap.dns.qtype_percentage.aaaa	14.201878	1760582504
dnstap.dns.qtype_percentage.srv	0.117371	1760582504
dnstap.dns.qtype_percentage.https	10.680751	1760582504
dnstap.dns.qtype_percentage.mx	0.117371	1760582504
dnstap.dns.qtype_percentage.txt	0.117371	1760582504
dnstap.dns.latency.mean	0.000000	1760582504
dnstap.dns.latency.p90	0.000000	1760582504
dnstap.dns.latency.p95	0.000000	1760582504
dnstap.dns.latency.p99	0.000000	1760582504
dnstap.dns.rcode.refused	0.000000	1760582504
dnstap.dns.rcode.noerror	25.636364	1760582504
dnstap.dns.rcode.servfail	0.000000	1760582504
dnstap.dns.rcode.nxdomain	0.181818	1760582504
```



## ライセンス

MIT License
