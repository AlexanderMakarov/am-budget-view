package bankdownload

import (
	"testing"

	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

func TestSourceByID(t *testing.T) {
	t.Parallel()

	def, ok := SourceByID(SourceAmeriaBusiness)
	if !ok {
		t.Fatalf("SourceByID(%q) ok = false, want true", SourceAmeriaBusiness)
	}
	if def.ID != SourceAmeriaBusiness {
		t.Errorf("def.ID = %q, want %q", def.ID, SourceAmeriaBusiness)
	}
	if !def.SupportsInAppDownload {
		t.Error("AmeriaBusiness should support in-app download")
	}

	if _, ok := SourceByID(SourceID("does-not-exist")); ok {
		t.Error("SourceByID for unknown id ok = true, want false")
	}
}

func TestSourceByID_CoversEveryRegisteredSource(t *testing.T) {
	t.Parallel()

	for _, def := range AllSources {
		if _, ok := SourceByID(def.ID); !ok {
			t.Errorf("SourceByID(%q) ok = false, want true", def.ID)
		}
	}
}

func TestMatchFileInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fi   model.FileInfo
		want SourceID
	}{
		{
			name: "nil source",
			fi:   model.FileInfo{Source: nil},
			want: "",
		},
		{
			name: "empty tag",
			fi:   model.FileInfo{Source: &model.TransactionsSource{Tag: ""}},
			want: "",
		},
		{
			name: "ameria csv prefix",
			fi:   model.FileInfo{Source: &model.TransactionsSource{Tag: "AmeriaCsv:1234"}},
			want: SourceAmeriaBusiness,
		},
		{
			name: "generic csv prefix",
			fi:   model.FileInfo{Source: &model.TransactionsSource{Tag: "GenericCsv:acc"}},
			want: SourceMyAmeriaGeneric,
		},
		{
			name: "inecobank xlsx card variant",
			fi:   model.FileInfo{Source: &model.TransactionsSource{Tag: "InecoExcelCard:42"}},
			want: SourceInecobankXLSX,
		},
		{
			name: "unknown prefix",
			fi:   model.FileInfo{Source: &model.TransactionsSource{Tag: "Totally:Unknown"}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchFileInfo(tt.fi); got != tt.want {
				t.Errorf("MatchFileInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}
