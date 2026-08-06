# Vulnerable fixtures for betterscan smoke tests

Hand-written snippets to exercise each scanner plugin:

| Path | Primary tool(s) | Bug class |
|------|-----------------|-----------|
| `java/Vuln.java` | **cpg** | NPE + OOB (CPG_NPE, CPG_OOB) |
| `c/bof.c` | **joern**, opengrep | `gets` / strcpy buffer overflow |
| `python/tainted.py` | **bandit**, opengrep | shell=True, eval, pickle, yaml.load |
| `go/` | **gostaticcheck**, opengrep | deprecated ioutil, path remove |
| `ruby/app.rb` | opengrep | system/backticks, SQL concat |
| `js/sqli.js` | opengrep | SQL + command injection |
| `php/sqli.php` | opengrep | SQL + command injection + XSS |
| `docker/Dockerfile` | **trivy** | USER root / weak image config |

## CLI examples

```bash
cd betterscan
./betterscan --code-dir ../fixtures/vulnerable --install-missing \
  --tools cpg,trivy,bandit,gostaticcheck,opengrep,joern \
  --json-out /tmp/betterscan-vuln.json --strategy sequential

# Deduplication scopes
./betterscan --code-dir ../fixtures/vulnerable --tools cpg,trivy \
  --dedupe-scope strict --json-out /tmp/strict.json
./betterscan --code-dir ../fixtures/vulnerable --tools cpg,trivy \
  --dedupe-scope file-line --json-out /tmp/file-line.json
```
