package uploads

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goirc/bot"
	"goirc/db/model"
	"goirc/events"
	"goirc/web/auth"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-chi/chi/v5"
	"golang.org/x/image/draw"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

var ErrNotSupported = errors.New("not a supported image format")

type service struct {
	Queries *model.Queries
	DB      *sql.DB
	Bot     *bot.Bot
}

func NewUploader(q *model.Queries, db *sql.DB, bot *bot.Bot) *service {
	return &service{Queries: q, DB: db, Bot: bot}
}

var snarfTimezoneJS = `fetch("/snarf-timezone", {method: "POST", headers: {"X-Timezone": Intl.DateTimeFormat().resolvedOptions().timeZone}})`

func (s *service) GetHandler(w http.ResponseWriter, r *http.Request) {
	nick := r.Context().Value(auth.NickKey).(string)

	const defaultPer = 100
	per, _ := strconv.ParseInt(r.URL.Query().Get("per"), 10, 64)
	if per <= 0 {
		per = defaultPer
	}
	page, _ := strconv.ParseInt(r.URL.Query().Get("page"), 10, 64)
	if page < 0 {
		page = 0
	}

	files, err := s.Queries.ListFiles(r.Context(), model.ListFilesParams{Limit: per, Offset: page * per})
	if err != nil {
		http.Error(w, fmt.Sprintf("ListFiles: %s", err), http.StatusInternalServerError)
		return
	}

	nodes := make([]Node, 0, len(files))
	for _, f := range files {
		full := fmt.Sprintf("/uploads/%d", f.ID)
		thumb := fmt.Sprintf("/uploads/%d/thumb", f.ID)
		var node Node

		if strings.HasPrefix(f.Mime.String, "audio/") {
			rng := rand.New(rand.NewPCG(uint64(f.ID), 0))
			node = Div(Style(fmt.Sprintf("display: flex; flex-direction: column; justify-content: center; width: 100%%; height: 100%%; background: linear-gradient(%ddeg, hsl(%d,70%%,60%%), hsl(%d,70%%,60%%));", rng.IntN(360), rng.IntN(360), rng.IntN(360))),
				Audio(Src(full), Controls(), Preload("none")),
			)
		} else if strings.HasPrefix(f.Mime.String, "video/") {
			rng := rand.New(rand.NewPCG(uint64(f.ID), 0))
			node = A(Href(full), Style(fmt.Sprintf("display: flex; flex-direction: column; justify-content: center; width: 100%%; height: 100%%; background: linear-gradient(%ddeg, hsl(%d,70%%,60%%), hsl(%d,70%%,60%%));", rng.IntN(360), rng.IntN(360), rng.IntN(360))),
				Video(Src(full), Controls(), Preload("metadata"), Style("width: 100%; height: 100%; object-fit: contain;"), Attr("onclick", "event.stopPropagation()"), Attr("onloadedmetadata", "this.currentTime=0.001")),
			)
		} else if f.Mime.String == "image/svg+xml" {
			node = A(Img(Src(full), Loading("lazy"), Style("width: 100%; height: 100%; object-fit: contain;")), Href(full))
		} else {
			node = A(Img(Src(thumb), Loading("lazy"), Style("width: 100%; height: 100%; object-fit: contain;")), Href(full))
		}
		node = Div(Style("display: flex; flex-direction: column; justify-content: center; width: 300px; height: 300px; margin: 4px; overflow: hidden; flex-shrink: 0; background: #eee;"), node)
		nodes = append(nodes, node)
	}

	HTML(
		Head(
			Script(Raw(snarfTimezoneJS)),
		),
		Body(
			Div(ID("dropzone"),
				H1(Text("annie file uploader")),
				P(Textf("hello, %s", nick)),
				Form(Method("POST"), Action("uploads"), EncType("multipart/form-data"),
					Input(Type("file"), Name("thefile")),
					Button(Text("Upload")),
					P(Textf("Links to uploaded files will be sent to %s.  You can also drag and drop or paste a file to upload.", s.Bot.Channel)),
				),
			),
			Div(Style("display: flex; gap: 1em; padding: 8px;"),
				If(page > 0, A(Href(fmt.Sprintf("?page=%d&per=%d", page-1, per)), Text("← newer"))),
				If(int64(len(files)) == per, A(Href(fmt.Sprintf("?page=%d&per=%d", page+1, per)), Text("older →"))),
			),
			Div(ID("image-index"), Style("display: flex; flex-wrap: wrap;"),
				Group(nodes),
			),
			Div(Style("display: flex; gap: 1em; padding: 8px;"),
				If(page > 0, A(Href(fmt.Sprintf("?page=%d&per=%d", page-1, per)), Text("← newer"))),
				If(int64(len(files)) == per, A(Href(fmt.Sprintf("?page=%d&per=%d", page+1, per)), Text("older →"))),
			),
			Script(Raw(`
const dropzone = document.getElementById('dropzone');
    // Handle drag events
    ['dragenter', 'dragover'].forEach(event => {
      dropzone.addEventListener(event, e => {
        e.preventDefault();
        dropzone.classList.add('hover');
      });
    });

    ['dragleave', 'drop'].forEach(event => {
      dropzone.addEventListener(event, e => {
        e.preventDefault();
        dropzone.classList.remove('hover');
      });
    });

    // Handle drop
    dropzone.addEventListener('drop', e => {
      const files = e.dataTransfer.files;
      if (files.length > 0) {
        uploadFile(files[0]);
      }
    });

 // Upload file using fetch
    function uploadFile(file) {
      const formData = new FormData();
      formData.append('thefile', file);

      const maxSize = 10 * 1024 * 1024; // 10MB in bytes
      if (file.size > maxSize) {
	alert('File is too large (max 10MB)');
	return;
      }

      fetch('/uploads', {
        method: 'POST',
        body: formData
      })
      .then(res => {
        if (res.redirected) {
          location.href = res.url;
        } else {
          location.href = '/uploads';
        }
      })
      .catch(err => {
        alert('upload failed');
        console.error(err);
      });
    }

document.addEventListener('paste', (e) => {
  const items = e.clipboardData.items;
  for (const item of items) {
    if (item.kind === 'file') {
      const file = item.getAsFile();
      if (file) {
        uploadFile(file);
      }
    }
  }
});
`)),
		),
	).Render(w)
}

func (s *service) PostHandler(w http.ResponseWriter, r *http.Request) {
	nick := r.Context().Value(auth.NickKey).(string)

	err := r.ParseMultipartForm(10 << 20) // 10 MB max memory
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	formFile, _, err := r.FormFile("thefile")
	if err != nil {
		http.Error(w, "Failed to retrieve file", http.StatusBadRequest)
		return
	}
	defer formFile.Close()

	data, err := io.ReadAll(formFile)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	file, err := s.Queries.InsertFile(r.Context(), model.InsertFileParams{
		Nick:    nick,
		Content: data,
	})
	if err != nil {
		http.Error(w, "Failed to save file to DB", http.StatusInternalServerError)
		return
	}

	mtype := mimetype.Detect(data)
	_ = s.Queries.UpdateFileMime(r.Context(), model.UpdateFileMimeParams{
		ID:   file.ID,
		Mime: sql.NullString{String: mtype.String(), Valid: true},
	})

	thumb, err := makeThumbnail(data)
	if err != nil && !errors.Is(err, ErrNotSupported) {
		log.Printf("makeThumbnail id=%d: %v", file.ID, err)
	}
	if thumb != nil {
		_ = s.Queries.UpdateFileThumbnail(r.Context(), model.UpdateFileThumbnailParams{
			ID:        file.ID,
			Thumbnail: thumb,
		})
	}

	err = s.Bot.Events.Insert(s.Bot.Channel, events.FileUploaded{Nick: nick, FileID: file.ID})
	if err != nil {
		http.Error(w, fmt.Sprintf("Events.Insert: %s", err), http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("%s/uploads/%d", os.Getenv("ROOT_URL"), file.ID)

	note, err := s.Queries.InsertNote(context.TODO(), model.InsertNoteParams{
		Target: s.Bot.Channel,
		Nick:   sql.NullString{String: nick, Valid: true},
		Kind:   "link",
		Text:   sql.NullString{String: url, Valid: true},
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("InsertNote: %s", err), http.StatusInternalServerError)
		return
	}

	s.Bot.Conn.Privmsgf(s.Bot.Channel, "%s uploaded %s", nick, note.Text.String)

	http.Redirect(w, r, "/uploads", http.StatusSeeOther)
}

func (s *service) FileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	file, err := s.Queries.GetFile(ctx, int64(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if file.Mime.Valid {
		w.Header().Set("Content-Type", file.Mime.String)
	}
	w.Write(file.Content)
}

func (s *service) ThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	thumb, err := s.Queries.GetFileThumbnail(ctx, int64(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(thumb) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(thumb)
}

func (s *service) SuccessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	file, err := s.Queries.GetFile(ctx, int64(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("/uploads/%d", file.ID)

	HTML(
		Div(Text("upload successful")),
		Div(A(Text(url), Href(url))),
		Div(A(Text("upload another"), Href("/uploads"))),
	).Render(w)
}

func (s *service) BackfillStatusHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Queries.ListFilesNeedingThumbnail(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list files: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "%d images need thumbnails\n", len(rows))
}

func (s *service) BackfillMimeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	rows, err := s.Queries.ListFilesNeedingMime(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("list files: %v", err), http.StatusInternalServerError)
		return
	}

	var count int
	for _, id := range rows {
		if ctx.Err() != nil {
			break
		}
		file, err := s.Queries.GetFile(ctx, id)
		if err != nil {
			log.Printf("backfill mime: get file %d: %v", id, err)
			continue
		}
		mtype := mimetype.Detect(file.Content)
		if err := s.Queries.UpdateFileMime(ctx, model.UpdateFileMimeParams{
			ID:   id,
			Mime: sql.NullString{String: mtype.String(), Valid: true},
		}); err != nil {
			log.Printf("backfill mime: update file %d: %v", id, err)
			continue
		}
		count++
	}

	fmt.Fprintf(w, "backfill mime: processed %d files\n", count)
}

func (s *service) BackfillHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	rows, err := s.Queries.ListFilesNeedingThumbnail(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("list files: %v", err), http.StatusInternalServerError)
		return
	}

	var count int
	for _, id := range rows {
		if ctx.Err() != nil {
			break
		}
		file, err := s.Queries.GetFile(ctx, id)
		if err != nil {
			log.Printf("backfill: get file %d: %v", id, err)
			continue
		}
		thumb, err := makeThumbnail(file.Content)
		if errors.Is(err, ErrNotSupported) {
			continue
		}
		if err != nil {
			log.Printf("backfill: thumbnail for file %d: %v", id, err)
			continue
		}
		if err := s.Queries.UpdateFileThumbnail(ctx, model.UpdateFileThumbnailParams{
			Thumbnail: thumb,
			ID:        id,
		}); err != nil {
			log.Printf("backfill: update file %d: %v", id, err)
			continue
		}
		count++
	}

	fmt.Fprintf(w, "backfill: processed %d images\n", count)
}

// makeThumbnail decodes an image and returns a JPEG thumbnail scaled to fit within 300x300.
// Returns ErrNotSupported if the data is not a recognized image format.
func makeThumbnail(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotSupported
	}

	const maxDim = 300
	bounds := src.Bounds()
	sw, sh := bounds.Dx(), bounds.Dy()

	tw, th := sw, sh
	if sw > maxDim || sh > maxDim {
		if sw > sh {
			tw = maxDim
			th = sh * maxDim / sw
		} else {
			th = maxDim
			tw = sw * maxDim / sh
		}
	}
	if tw == 0 {
		tw = 1
	}
	if th == 0 {
		th = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
