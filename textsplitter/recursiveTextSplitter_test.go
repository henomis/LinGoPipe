package textsplitter

import (
	"reflect"
	"testing"

	"github.com/henomis/lingoose/document"
	"github.com/henomis/lingoose/types"
)

func TestRecursiveCharacterTextSplitter_SplitDocuments(t *testing.T) {
	type fields struct {
		textSplitter textSplitter
		separators   []string
	}
	type args struct {
		documents []document.Document
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []document.Document
	}{
		{
			name: "TestRecursiveCharacterTextSplitter_SplitDocuments",
			fields: fields{
				textSplitter: textSplitter{
					chunkSize:    10,
					chunkOverlap: 0,
					lengthFunction: func(s string) int {
						return len(s)
					},
				},
				separators: []string{"\n\n", "\n", " ", ""},
			},
			args: args{
				documents: []document.Document{
					{
						Content:  "This is a test",
						Metadata: types.Meta{},
					},
				},
			},
			want: []document.Document{
				{
					Content:  "This is a",
					Metadata: types.Meta{},
				},
				{
					Content:  "test",
					Metadata: types.Meta{},
				},
			},
		},
		{
			name: "TestRecursiveCharacterTextSplitter_SplitDocuments",
			fields: fields{
				textSplitter: textSplitter{
					chunkSize:    10,
					chunkOverlap: 0,
					lengthFunction: func(s string) int {
						return len(s)
					},
				},
				separators: []string{"\n\n", "\n", " ", ""},
			},
			args: args{
				documents: []document.Document{
					{
						Content: "This is a test",
						Metadata: types.Meta{
							"test":  "test",
							"test2": "test2",
						},
					},
				},
			},
			want: []document.Document{
				{
					Content: "This is a",
					Metadata: types.Meta{
						"test":  "test",
						"test2": "test2",
					},
				},
				{
					Content: "test",
					Metadata: types.Meta{
						"test":  "test",
						"test2": "test2",
					},
				},
			},
		},
		{
			name: "TestRecursiveCharacterTextSplitter_SplitDocuments2",
			fields: fields{
				textSplitter: textSplitter{
					chunkSize:    20,
					chunkOverlap: 5,
					lengthFunction: func(s string) int {
						return len(s)
					},
				},
				separators: []string{"\n\n", "\n", " ", ""},
			},
			args: args{
				documents: []document.Document{
					{
						Content:  "Lorem ipsum dolor sit amet,\n\nconsectetur adipisci elit",
						Metadata: types.Meta{},
					},
				},
			},
			want: []document.Document{
				{
					Content:  "Lorem ipsum dolor",
					Metadata: types.Meta{},
				},
				{
					Content:  "dolor sit amet,",
					Metadata: types.Meta{},
				},
				{
					Content:  "consectetur adipisci",
					Metadata: types.Meta{},
				},
				{
					Content:  "elit",
					Metadata: types.Meta{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recursiveCharacterTextSplitter{
				textSplitter: tt.fields.textSplitter,
				separators:   tt.fields.separators,
			}
			if got := r.SplitDocuments(tt.args.documents); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RecursiveCharacterTextSplitter.SplitDocuments() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
