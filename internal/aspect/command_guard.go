package aspect

import (
	"regexp"
)

// GuardStatus 表示命令防护的检查结果
type GuardStatus string

const (
	GuardStatusAllowed       GuardStatus = "allowed"
	GuardStatusBlocked       GuardStatus = "blocked"
	GuardStatusNeedsApproval GuardStatus = "needs_approval"
)

// FileOp 表示文件操作类型
type FileOp string

const (
	FileOpRead  FileOp = "read"
	FileOpWrite FileOp = "write"
)

// GuardResult 是防护检查的结果
type GuardResult struct {
	Status  GuardStatus
	Reason  string // 拦截原因
	Pattern string // 匹配的规则
}

// CommandGuard 命令防护器，实现双层防御（黑名单 + 白名单）
type CommandGuard struct {
	blacklistPatterns    []*regexp.Regexp
	hitlPatterns         []*regexp.Regexp    // 需人工审批
	readOnlyWhitelist    []*regexp.Regexp    // 安全只读命令
	shellFeaturePatterns []*regexp.Regexp    // shell 特性拦截
	configFilePatterns   []*regexp.Regexp    // 配置文件写入需审批
}

// NewCommandGuard 创建一个新的命令防护器
func NewCommandGuard() *CommandGuard {
	g := &CommandGuard{}

	// 黑名单：四条铁律
	g.blacklistPatterns = []*regexp.Regexp{
		regexp.MustCompile(`rm\s+-rf\s+/`),                 // rm -rf / (and any subpath)
		regexp.MustCompile(`rm\s+-rf\s+~`),                // rm -rf ~
		regexp.MustCompile(`kill\s+-9\s+1\b`),             // kill -9 1
		regexp.MustCompile(`kill\s+-9\s+\$PPID`),          // kill -9 $PPID
		regexp.MustCompile(`drop\s+database\b`),           // drop database
		regexp.MustCompile(`redis-cli\s+flushall`),        // redis-cli flushall
		regexp.MustCompile(`:\(\)\s*\{\s*:\|:&\s*\}\s*;:`), // fork bomb
		regexp.MustCompile(`rm\s+.*\.sqlite`),             // 删除 sqlite
		regexp.MustCompile(`rm\s+.*\.rdb`),                // 删除 rdb
	}

	// HITL：需人工审批的操作
	g.hitlPatterns = []*regexp.Regexp{
		regexp.MustCompile(`git\s+push\s+(-f|--force)`),
		regexp.MustCompile(`git\s+push\s+.*\s+(-f|--force)`),
		regexp.MustCompile(`go\s+get\b`),
		regexp.MustCompile(`npm\s+install\b`),
		regexp.MustCompile(`go\s+install\b`),
	}

	// 安全只读命令白名单
	g.readOnlyWhitelist = []*regexp.Regexp{
		regexp.MustCompile(`^ls\b`),
		regexp.MustCompile(`^cat\b`),
		regexp.MustCompile(`^echo\b`),
		regexp.MustCompile(`^git\s+(status|log|diff|branch)\b`),
		regexp.MustCompile(`^go\s+(test|build|vet|fmt)\b`),
		regexp.MustCompile(`^pwd\b`),
		regexp.MustCompile(`^head\b`),
		regexp.MustCompile(`^tail\b`),
		regexp.MustCompile(`^grep\b`),
		regexp.MustCompile(`^find\b`),
	}

	// Shell 特性拦截
	g.shellFeaturePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\|`),    // pipe
		regexp.MustCompile(`;`),     // semicolon
		regexp.MustCompile(`>`),     // redirect output
		regexp.MustCompile(`<`),     // redirect input
		regexp.MustCompile(`\$\(`),  // command substitution
		regexp.MustCompile("`"),     // backtick
		regexp.MustCompile(`\$\w+`), // variable expansion
		regexp.MustCompile(`\$\{`),  // variable expansion ${}
		regexp.MustCompile(`&\s*$`), // background process
		regexp.MustCompile(`\|\s*&`), // pipe to background
	}

	// 配置文件写入需审批
	g.configFilePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^\.env$`),
		regexp.MustCompile(`^\.env\.`),
		regexp.MustCompile(`config\.json$`),
		regexp.MustCompile(`go\.mod$`),
		regexp.MustCompile(`go\.sum$`),
		regexp.MustCompile(`\.yaml$`),
		regexp.MustCompile(`\.yml$`),
	}

	return g
}

// GuardCommand 检查命令：黑名单 → shell特性 → HITL → 白名单 → 允许
func (g *CommandGuard) GuardCommand(cmd string) GuardResult {
	// 1. 检查黑名单
	for _, p := range g.blacklistPatterns {
		if p.MatchString(cmd) {
			return GuardResult{
				Status:  GuardStatusBlocked,
				Reason:  "命令匹配黑名单规则，违反安全铁律",
				Pattern: p.String(),
			}
		}
	}

	// 2. 检查 shell 特性（fork bomb 已在黑名单中检查）
	for _, p := range g.shellFeaturePatterns {
		if p.MatchString(cmd) {
			return GuardResult{
				Status:  GuardStatusBlocked,
				Reason:  "命令包含 shell 特性（管道/重定向/变量展开/后台等），已被拦截",
				Pattern: p.String(),
			}
		}
	}

	// 3. 检查 HITL 模式
	for _, p := range g.hitlPatterns {
		if p.MatchString(cmd) {
			return GuardResult{
				Status:  GuardStatusNeedsApproval,
				Reason:  "命令属于高风险操作，需要人工审批",
				Pattern: p.String(),
			}
		}
	}

	// 4. 检查白名单
	for _, p := range g.readOnlyWhitelist {
		if p.MatchString(cmd) {
			return GuardResult{Status: GuardStatusAllowed}
		}
	}

	// 5. 默认允许（不在任何列表中的命令）
	return GuardResult{Status: GuardStatusAllowed}
}

// GuardFilePath 验证文件操作
func (g *CommandGuard) GuardFilePath(path string, op FileOp) GuardResult {
	// 只读操作总是允许
	if op == FileOpRead {
		return GuardResult{Status: GuardStatusAllowed}
	}

	// 写操作：检查是否是配置文件
	for _, p := range g.configFilePatterns {
		if p.MatchString(path) {
			return GuardResult{
				Status:  GuardStatusNeedsApproval,
				Reason:  "写入配置文件需要人工审批",
				Pattern: p.String(),
			}
		}
	}

	return GuardResult{Status: GuardStatusAllowed}
}

// IsShellFeature 检查命令是否包含 shell 特性
func (g *CommandGuard) IsShellFeature(cmd string) bool {
	for _, p := range g.shellFeaturePatterns {
		if p.MatchString(cmd) {
			return true
		}
	}
	return false
}
