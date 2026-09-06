package websearch

import (
	"testing"
)

func TestAllEngines(t *testing.T) {
	engines := AllEngines()

	if _, ok := engines[CategoryText]; !ok {
		t.Errorf("CategoryText missing")
	}
	if _, ok := engines[CategoryBooks]; !ok {
		t.Errorf("CategoryBooks missing")
	}

	foundGrok := false
	for _, e := range engines[CategoryText] {
		if e.Name() == "grokipedia" {
			foundGrok = true
			break
		}
	}
	if !foundGrok {
		t.Errorf("Grokipedia not found in CategoryText")
	}

	foundAnna := false
	for _, e := range engines[CategoryBooks] {
		if e.Name() == "annasarchive" {
			foundAnna = true
			break
		}
	}
	if !foundAnna {
		t.Errorf("AnnasArchive not found in CategoryBooks")
	}
}

func TestAnnasArchiveExtractsCommentWrappedBooks(t *testing.T) {
	engine, ok := NewAnnasArchive().(*XPathEngine)
	if !ok {
		t.Fatalf("expected XPathEngine")
	}
	html := []byte(`<!--
		<div class="record-list-outer">
			<div>
				<a class="text-lg" href="/md5/abc123">The Sea-Wolf</a>
				<a><span class="user"></span>Jack London</a>
				<a><span class="company"></span>DigiCat, 2022</a>
				<div class="text-gray-800">English [en], .epub</div>
				<img src="https://example.test/cover.jpg"/>
			</div>
		</div>
	-->`)
	if engine.PreProcess != nil {
		html = engine.PreProcess(html)
	}
	results, err := engine.extractResults(html)
	if err != nil {
		t.Fatalf("extractResults failed: %v", err)
	}
	if len(results) != 1 || results[0].Books == nil {
		t.Fatalf("expected one book result, got %#v", results)
	}
	book := results[0].Books
	if book.Title != "The Sea-Wolf" || book.Author != "Jack London" || book.URL != "/md5/abc123" {
		t.Fatalf("unexpected book result: %#v", book)
	}
}

func TestAnnasArchiveBackendSelectsBookEngineEvenFromTextCategory(t *testing.T) {
	engines := selectEngines(CategoryText, "annasarchive")
	if len(engines) != 1 {
		t.Fatalf("expected one selected engine, got %#v", engines)
	}
	if engines[0].Name() != "annasarchive" || engines[0].Category() != CategoryBooks {
		t.Fatalf("expected annasarchive book engine, got %s/%s", engines[0].Name(), engines[0].Category())
	}
}
