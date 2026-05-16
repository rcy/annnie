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
	"goirc/internal/og"
	"goirc/web/auth"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
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

	type mediaItem struct {
		CreatedAt     time.Time
		FullURL       string
		ThumbURL      string
		Mime          string
		Text          string
		Kind          string
		OgTitle       string
		OgDescription string
		OgImage       string
	}

	files, err := s.Queries.ListAllFiles(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("ListAllFiles: %s", err), http.StatusInternalServerError)
		return
	}
	genImages, err := s.Queries.GeneratedImages(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("GeneratedImages: %s", err), http.StatusInternalServerError)
		return
	}

	notes, err := s.Queries.ListAllNotes(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("ListAllNotes: %s", err), http.StatusInternalServerError)
		return
	}

	items := make([]mediaItem, 0, len(files)+len(genImages)+len(notes))
	for _, f := range files {
		items = append(items, mediaItem{
			CreatedAt: f.CreatedAt,
			FullURL:   fmt.Sprintf("/uploads/%d", f.ID),
			ThumbURL:  fmt.Sprintf("/uploads/%d/thumb", f.ID),
			Mime:      f.Mime.String,
		})
	}
	for _, g := range genImages {
		items = append(items, mediaItem{
			CreatedAt: g.CreatedAt,
			FullURL:   fmt.Sprintf("/i/%d", g.ID),
			ThumbURL:  fmt.Sprintf("/images/%d.png", g.ID),
			Mime:      "image/png",
		})
	}
	uploadPrefix := os.Getenv("ROOT_URL") + "/uploads/"
	for _, n := range notes {
		if n.Kind == "link" && strings.HasPrefix(n.Text.String, uploadPrefix) {
			continue
		}
		fullURL := fmt.Sprintf("/note/%d", n.ID)
		if n.Kind == "link" {
			fullURL = n.Text.String
		}
		item := mediaItem{
			CreatedAt:     n.CreatedAt,
			FullURL:       fullURL,
			Kind:          n.Kind,
			Text:          n.Text.String,
			OgTitle:       n.OgTitle.String,
			OgDescription: n.OgDescription.String,
			OgImage:       n.OgImage.String,
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := len(items)
	start := int(page * per)
	if start > total {
		start = total
	}
	end := start + int(per)
	if end > total {
		end = total
	}
	hasMore := end < total
	items = items[start:end]

	nodes := make([]Node, 0, len(items))
	for _, f := range items {
		var node Node
		if strings.HasPrefix(f.Mime, "audio/") {
			rng := rand.New(rand.NewPCG(uint64(f.CreatedAt.UnixNano()), 0))
			node = Div(Style(fmt.Sprintf("display: flex; flex-direction: column; justify-content: center; width: 100%%; height: 100%%; background: linear-gradient(%ddeg, hsl(%d,70%%,60%%), hsl(%d,70%%,60%%));", rng.IntN(360), rng.IntN(360), rng.IntN(360))),
				Audio(Src(f.FullURL), Controls(), Preload("none")),
			)
		} else if strings.HasPrefix(f.Mime, "video/") {
			node = A(Href(f.FullURL), Style("display: flex; width: 100%; height: 100%;"),
				Video(Src(f.FullURL), Controls(), Preload("none"), Attr("poster", f.ThumbURL), Style("width: 100%; height: 100%; object-fit: contain;"), Attr("onclick", "event.stopPropagation()")),
			)
		} else if f.Mime == "image/svg+xml" {
			node = A(Img(Src(f.FullURL), Loading("lazy"), Style("width: 100%; height: 100%; object-fit: contain;")), Href(f.FullURL))
		} else if f.Kind != "" {
			if f.OgImage != "" {
				faviconURL := ""
				if u, err := url.Parse(f.FullURL); err == nil && u.Host != "" {
					faviconURL = "https://www.google.com/s2/favicons?domain=" + u.Hostname() + "&sz=32"
				}
				node = A(Href(f.FullURL), Style("display: block; position: relative; width: 100%; height: 100%;"),
					Img(Src(f.OgImage), Loading("lazy"), Style("width: 100%; height: 100%; object-fit: cover;")),
					If(faviconURL != "", Div(Style("position: absolute; bottom: 8px; right: 8px; width: 28px; height: 28px; background: rgba(255,255,255,0.85); border-radius: 6px; display: flex; align-items: center; justify-content: center; pointer-events: none;"),
						Img(Src(faviconURL), Style("width: 20px; height: 20px;")),
					)),
				)
			} else {
				var displayText, bg string

				switch f.Kind {
				case "link":
					bg = "#222"
					displayText = f.OgTitle
					if f.OgTitle == "error" {
						displayText = f.Text
						bg = "#522"
					}
				case "quote", "note":
					displayText = f.Text
					var h uint64
					for i, c := range f.Text {
						h += uint64(c) * uint64(i+1)
					}
					rng := rand.New(rand.NewPCG(h, 0))
					bg = fmt.Sprintf("hsl(%d,60%%,22%%)", rng.IntN(360))
				default:
					displayText = f.Text
				}

				faviconURL := ""
				if u, err := url.Parse(f.FullURL); err == nil && u.Host != "" {
					faviconURL = "https://www.google.com/s2/favicons?domain=" + u.Hostname() + "&sz=32"
				}
				node = A(Href(f.FullURL), Style(fmt.Sprintf("display: flex; position: relative; align-items: center; justify-content: center; width: 100%%; height: 100%%; padding: 12px; box-sizing: border-box; background: %s; color: #eee; text-decoration: none; overflow: hidden;", bg)),
					P(Attr("data-fittext", ""), Style("margin: 0; font-weight: bold; line-height: 1.2; overflow: hidden; width: 100%;"), Text(displayText)),
					If(faviconURL != "", Div(Style("position: absolute; bottom: 8px; right: 8px; width: 28px; height: 28px; background: rgba(255,255,255,0.85); border-radius: 6px; display: flex; align-items: center; justify-content: center; pointer-events: none;"),
						Img(Src(faviconURL), Style("width: 20px; height: 20px;")),
					)),
				)
			}
		} else {
			node = A(Img(Src(f.ThumbURL), Loading("lazy"), Style("width: 100%; height: 100%; object-fit: contain;")), Href(f.FullURL))
		}
		node = Div(Style("display: flex; flex-direction: column; justify-content: center; width: 300px; height: 300px; margin: 4px; overflow: hidden; flex-shrink: 0; background: #eee;"), node)
		nodes = append(nodes, node)
	}

	HTML(
		Head(
			Script(Raw(snarfTimezoneJS)),
		),
		Body(
			Div(ID("dropzone"), Style("display: flex; flex-direction: column; gap: 1em"),
				Code(
					Textf("<annnie> %s: links to uploaded files will be sent to %s.  you can also drag and drop or paste a file to upload. ", nick, s.Bot.Channel),
					A(Href("/old"), Text(" old")),
				),
				Form(Method("POST"), Action("uploads"), EncType("multipart/form-data"),
					Input(Type("file"), Name("thefile")),
					Button(Text("Upload")),
				),
			),
			Div(ID("image-index"), Style("display: flex; flex-wrap: wrap;"),
				Group(nodes),
			),
			If(hasMore, A(Href(fmt.Sprintf("?page=%d&per=%d", page+1, per)), Style("display: block; text-align: center; padding: 16px; margin: 16px 8px 48px; background: #333; color: #eee; text-decoration: none; border-radius: 4px;"), Text("older →"))),
			Script(Raw(`
const overlay = document.createElement('div');
overlay.style.cssText = 'display:none;position:fixed;inset:0;background:rgba(0,0,0,0.5);z-index:9999;align-items:center;justify-content:center;font-size:2em;color:#fff;pointer-events:none;';
overlay.textContent = 'drop the slop';
document.body.appendChild(overlay);

let dragCount = 0;
document.addEventListener('dragenter', e => {
  e.preventDefault();
  if (++dragCount === 1) overlay.style.display = 'flex';
});
document.addEventListener('dragleave', e => {
  if (--dragCount === 0) overlay.style.display = 'none';
});
document.addEventListener('dragover', e => { e.preventDefault(); });
document.addEventListener('drop', e => {
  e.preventDefault();
  dragCount = 0;
  overlay.style.display = 'none';
  const files = e.dataTransfer.files;
  if (files.length > 0) uploadFile(files[0]);
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
          location.href = '/';
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

document.querySelectorAll('[data-fittext]').forEach(el => {
  const parent = el.parentElement;
  const style = getComputedStyle(parent);
  const availW = parent.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
  const availH = parent.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom);
  let size = 128;
  el.style.fontSize = size + 'px';
  while ((el.scrollHeight > availH || el.scrollWidth > availW) && size > 10) {
    el.style.fontSize = --size + 'px';
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

	var thumb []byte
	if strings.HasPrefix(mtype.String(), "video/") || mtype.String() == "image/avif" {
		thumb, err = makeFFmpegThumbnail(data)
		if err != nil {
			log.Printf("makeFFmpegThumbnail id=%d: %v", file.ID, err)
		}
	} else {
		thumb, err = makeThumbnail(data)
		if err != nil && !errors.Is(err, ErrNotSupported) {
			log.Printf("makeThumbnail id=%d: %v", file.ID, err)
		}
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

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
		Div(A(Text("upload another"), Href("/"))),
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
		var thumb []byte
		if strings.HasPrefix(file.Mime.String, "video/") || file.Mime.String == "image/avif" {
			thumb, err = makeFFmpegThumbnail(file.Content)
		} else {
			thumb, err = makeThumbnail(file.Content)
			if errors.Is(err, ErrNotSupported) {
				continue
			}
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

func (s *service) BackfillOGHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	rows, err := s.Queries.ListLinkNotesNeedingOG(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("list notes: %v", err), http.StatusInternalServerError)
		return
	}

	var count int
	for _, row := range rows {
		if ctx.Err() != nil {
			break
		}
		data, err := og.Fetch(ctx, row.Text.String)
		if err != nil {
			data.Title = sql.NullString{String: "error", Valid: true}
			data.Description = sql.NullString{String: err.Error(), Valid: true}
		}
		if err := s.Queries.UpdateNoteOG(ctx, model.UpdateNoteOGParams{
			ID:            row.ID,
			OgTitle:       data.Title,
			OgDescription: data.Description,
			OgImage:       data.Image,
		}); err != nil {
			http.Error(w, fmt.Sprintf("backfill og: update note %d: %v", row.ID, err), http.StatusInternalServerError)
			return
		}
		count++
	}

	fmt.Fprintf(w, "backfill og: processed %d notes\n", count)
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

func makeFFmpegThumbnail(data []byte) ([]byte, error) {
	in, err := os.CreateTemp("", "video-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(in.Name())
	if _, err := in.Write(data); err != nil {
		in.Close()
		return nil, err
	}
	in.Close()

	out, err := os.CreateTemp("", "thumb-*.jpg")
	if err != nil {
		return nil, err
	}
	out.Close()
	defer os.Remove(out.Name())

	cmd := exec.Command("ffmpeg", "-i", in.Name(), "-vframes", "1", "-vf", "scale=300:300:force_original_aspect_ratio=decrease", "-y", "-loglevel", "error", out.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, output)
	}
	return os.ReadFile(out.Name())
}
