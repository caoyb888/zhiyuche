package dept

import (
	"testing"

	"github.com/google/uuid"
)

func TestChildPath(t *testing.T) {
	root := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	child := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rp := ChildPath("", root)
	if rp != "/11111111-1111-1111-1111-111111111111/" {
		t.Fatalf("root path = %q", rp)
	}
	cp := ChildPath(rp, child)
	if cp != "/11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/" {
		t.Fatalf("child path = %q", cp)
	}
	if !InSubtree(cp, rp) || !InSubtree(rp, rp) {
		t.Fatal("child and self must be inside the subtree")
	}
	if InSubtree(rp, cp) {
		t.Fatal("parent must not be inside the child's subtree")
	}
	// 同前缀但不同 uuid 不应误判（'/' 结尾保证整段匹配）
	other := ChildPath("", uuid.MustParse("11111111-1111-1111-1111-111111111112"))
	if InSubtree(other, rp) {
		t.Fatal("sibling with similar id must not match")
	}
}

func TestRebasePath(t *testing.T) {
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	old := ChildPath(ChildPath("", a), b) // /a/b/
	leaf := ChildPath(old, c)            // /a/b/c/
	newPrefix := ChildPath(ChildPath("", d), b)
	got := RebasePath(leaf, old, newPrefix)
	if got != ChildPath(newPrefix, c) {
		t.Fatalf("rebase = %q, want %q", got, ChildPath(newPrefix, c))
	}
	if RebasePath("/x/", old, newPrefix) != "/x/" {
		t.Fatal("unrelated path must be untouched")
	}
}

func TestBuildTree(t *testing.T) {
	hq, sales, dev, devA, devB, orphan := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	missing := uuid.New()
	in := []Dept{
		{ID: devB, ParentID: &dev, Name: "研发二组", Sort: 2},
		{ID: devA, ParentID: &dev, Name: "研发一组", Sort: 1},
		{ID: sales, ParentID: &hq, Name: "销售部", Sort: 5},
		{ID: dev, ParentID: &hq, Name: "研发部", Sort: 5}, // 同 sort 按 name：研发部 < 销售部
		{ID: hq, Name: "总部", Sort: 0},
		{ID: orphan, ParentID: &missing, Name: "孤儿", Sort: 9},
	}
	roots := BuildTree(in)
	if len(roots) != 2 {
		t.Fatalf("roots = %d, want 2 (总部 + 孤儿)", len(roots))
	}
	if roots[0].ID != hq || roots[1].ID != orphan {
		t.Fatalf("root order wrong: %s, %s", roots[0].Name, roots[1].Name)
	}
	if len(roots[0].Children) != 2 || roots[0].Children[0].ID != dev || roots[0].Children[1].ID != sales {
		t.Fatalf("hq children wrong: %+v", roots[0].Children)
	}
	devNode := roots[0].Children[0]
	if len(devNode.Children) != 2 || devNode.Children[0].ID != devA || devNode.Children[1].ID != devB {
		t.Fatalf("dev children wrong")
	}
	if devNode.Children[0].Children == nil {
		t.Fatal("leaf children must be an empty array, not null")
	}
	if got := BuildTree(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty input must give empty (non-nil) slice, got %#v", got)
	}
}
