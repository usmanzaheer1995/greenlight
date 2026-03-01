package data

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/usmanzaheer1995/greenlight/internal/testutil"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	var cleanup func()
	testDB, cleanup = testutil.StartPostgres(applyMoviesMigrations)
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func applyMoviesMigrations(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS movies (
                     id         bigserial PRIMARY KEY,
                    created_at timestamp(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
                     title      text        NOT NULL,
                    year       integer     NOT NULL,
                     runtime    integer     NOT NULL,
                    genres     text[]      NOT NULL,
                     version    integer     NOT NULL DEFAULT 1
            )`,
		// migration 000002
		`ALTER TABLE movies ADD CONSTRAINT movies_runtime_check CHECK (runtime >= 0)`,
		`ALTER TABLE movies ADD CONSTRAINT movies_year_check    CHECK (year BETWEEN 1888 AND date_part('year', now()))`,
		`ALTER TABLE movies ADD CONSTRAINT genres_length_check  CHECK (array_length(genres, 1) BETWEEN 1 AND 5)`,
		// migration 000003
		`CREATE INDEX IF NOT EXISTS movies_title_idx  ON movies USING GIN (to_tsvector('simple', title))`,
		`CREATE INDEX IF NOT EXISTS movies_genres_idx ON movies USING GIN (genres)`,
	}

	for _, s := range statements {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	return nil
}

func newTestMovieModel() MovieModel {
	return MovieModel{DB: testDB}
}

func truncateMovies(t *testing.T) {
	t.Helper()
	if _, err := testDB.Exec("TRUNCATE TABLE movies RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncateMovies: %v", err)
	}
}

func seedMovie(t *testing.T, m MovieModel, movie *Movie) *Movie {
	t.Helper()
	if err := m.Insert(movie); err != nil {
		t.Fatalf("seedMovie: Insert failed: %v", err)
	}
	return movie
}

func TestMovieModel_Insert(t *testing.T) {
	truncateMovies(t)

	model := newTestMovieModel()

	movie := &Movie{
		Title:   "The Dark Knight",
		Year:    2008,
		Runtime: 152,
		Genres:  []string{"action", "drama"},
	}

	err := model.Insert(movie)
	if err != nil {
		t.Fatalf("Insert() returned error: %v", err)
	}

	if movie.ID == 0 {
		t.Errorf("expected movie.ID to be populated after insert, got 0")
	}
	if movie.Version != 1 {
		t.Errorf("expected movie.Version = 1 after insert, got %d", movie.Version)
	}
	if movie.CreatedAt.IsZero() {
		t.Errorf("expected movie.CreatedAt to be set after insert, got zero value")
	}
}

func TestMovieModel_Get(t *testing.T) {
	truncateMovies(t)
	model := newTestMovieModel()

	seeded := seedMovie(t, model, &Movie{
		Title:   "Inception",
		Year:    2010,
		Runtime: 148,
		Genres:  []string{"sci-fi", "thriller"},
	})

	t.Run("found", func(t *testing.T) {
		got, err := model.Get(seeded.ID)
		if err != nil {
			t.Fatalf("Get(%d) unexpected error: %v", seeded.ID, err)
		}
		if got.ID != seeded.ID {
			t.Fatalf("Expected ID: %d; Actual: %d", seeded.ID, got.ID)
		}
		if got.Title != seeded.Title {
			t.Fatalf("Expected Title: %v; Actual: %v", seeded.Title, got.Title)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := model.Get(999_999)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Get(999999): want ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		_, err := model.Get(0)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Get(0): want ErrRecordNotFound, got %v", err)
		}
	})
}

// Get All
func TestMovieModel_GetAll(t *testing.T) {
	truncateMovies(t)
	model := newTestMovieModel()

	for _, mv := range []*Movie{
		{Title: "Alien", Year: 1979, Runtime: 117, Genres: []string{"sci-fi", "horror"}},
		{Title: "Aliens", Year: 1986, Runtime: 137, Genres: []string{"sci-fi", "action"}},
		{Title: "Blade Runner", Year: 1982, Runtime: 117, Genres: []string{"sci-fi", "drama"}},
		{Title: "The Shining", Year: 1980, Runtime: 146, Genres: []string{"horror"}},
		{Title: "RoboCop", Year: 1987, Runtime: 102, Genres: []string{"action", "sci-fi"}},
	} {
		seedMovie(t, model, mv)
	}

	defaultFilters := func() Filters {
		return Filters{
			Page:         1,
			PageSize:     20,
			Sort:         "id",
			SortSafeList: []string{"id", "title", "year", "-id", "-title", "-year"},
		}
	}

	t.Run("no filters returns all", func(t *testing.T) {
		got, meta, err := model.GetAll("", nil, defaultFilters())
		if err != nil {
			t.Fatalf("GetAll() error: %v", err)
		}
		if len(got) != 5 {
			t.Errorf("want 5 movies, got %d", len(got))
		}
		if meta.TotalRecords != 5 {
			t.Errorf("TotalRecords: want 5, got %d", meta.TotalRecords)
		}
	})

	t.Run("title search matches partial word", func(t *testing.T) {
		got, _, err := model.GetAll("Alien", nil, defaultFilters())
		if err != nil {
			t.Fatalf("GetAll(title='Alien') error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("title search 'Alien': want 2 results, got %d", len(got))
		}
	})

	t.Run("genre filter", func(t *testing.T) {
		got, _, err := model.GetAll("", []string{"horror"}, defaultFilters())
		if err != nil {
			t.Fatalf("GetAll(genre='horror') error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("genre 'horror': want 2 results, got %d", len(got))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		f := defaultFilters()
		f.PageSize = 2
		f.Page = 2

		got, meta, err := model.GetAll("", nil, f)
		if err != nil {
			t.Fatalf("GetAll(page=2, pageSize=2) error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("page 2: want 2 rows, got %d", len(got))
		}
		if meta.CurrentPage != 2 {
			t.Errorf("CurrentPage: want 2, got %d", meta.CurrentPage)
		}
		if meta.LastPage != 3 {
			t.Errorf("LastPage: want 3 (ceil(5/2)), got %d", meta.LastPage)
		}
	})

	t.Run("sort by year descending", func(t *testing.T) {
		f := defaultFilters()
		f.Sort = "-year"

		got, _, err := model.GetAll("", nil, f)
		if err != nil {
			t.Fatalf("GetAll(sort='-year') error: %v", err)
		}
		if got[0].Year != 1987 {
			t.Errorf("first result with -year sort: want 1987, got %d", got[0].Year)
		}
	})
}
