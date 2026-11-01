package main

import (
	"analytics-playground/internal/metabase"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func password(r *bufio.Reader) (string, error) {
	// stty keeps interactive credentials out of terminal echo; fail closed when unavailable.
	cmd := exec.Command("stty", "-echo")
	cmd.Stdin = os.Stdin
	if e := cmd.Run(); e != nil {
		return "", fmt.Errorf("パスワードを非表示で入力できません: %w", e)
	}
	defer func() {
		restore := exec.Command("stty", "echo")
		restore.Stdin = os.Stdin
		_ = restore.Run()
		fmt.Println()
	}()
	s, e := r.ReadString('\n')
	return strings.TrimSpace(s), e
}
func run() error {
	r := bufio.NewReader(os.Stdin)
	fmt.Print("Metabase 管理者メールアドレス: ")
	user, e := r.ReadString('\n')
	if e != nil {
		return e
	}
	fmt.Print("パスワード: ")
	pass, e := password(r)
	if e != nil {
		return e
	}
	url := os.Getenv("METABASE_URL")
	if url == "" {
		url = "http://localhost:3000"
	}
	c := metabase.Client{URL: url}
	if e = c.Login(strings.TrimSpace(user), pass); e != nil {
		return e
	}
	dbs, e := c.Databases()
	if e != nil {
		return e
	}
	for _, d := range dbs {
		if d.Engine == "postgres" {
			fmt.Printf("%d: %s\n", d.ID, d.Name)
		}
	}
	fmt.Print("分析DBのID: ")
	s, e := r.ReadString('\n')
	if e != nil {
		return e
	}
	db, e := strconv.Atoi(strings.TrimSpace(s))
	if e != nil {
		return e
	}
	id, e := c.CreateDashboard(db)
	if e != nil {
		return e
	}
	fmt.Printf("作成しました: %s/dashboard/%d\n再実行すると別のダッシュボードが作成されます。\n", strings.TrimRight(url, "/"), id)
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
