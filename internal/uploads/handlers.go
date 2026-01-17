package uploads

import (
	"context"
	"database/sql"
	"fmt"
	"goirc/bot"
	"goirc/db/model"
	"goirc/events"
	"goirc/web/auth"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

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

	HTML(
		Head(
			Script(Raw(snarfTimezoneJS)),
		),
		Body(
			Div(ID("dropzone"), Style("height: 100vh;"),
				H1(Text("annie file uploader")),
				P(Textf("hello, %s", nick)),
				Form(Method("POST"), Action("uploads"), EncType("multipart/form-data"),
					Input(Type("file"), Name("thefile")),
					Button(Text("Upload")),
					P(Textf("Links to uploaded files will be sent to %s.  You can also drag and drop or paste a file to upload.", s.Bot.Channel)),
				),
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
        headers: { "Accept": "application/json" },
        method: 'POST',
        body: formData
      })
      .then(res => res.text())
      .then(text => {
        location.href = text;
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

	redirectURL := fmt.Sprintf("/uploads/success/%d", file.ID)
	w.Write([]byte(redirectURL))
	http.Redirect(w, r, fmt.Sprintf(redirectURL, file.ID), http.StatusSeeOther)
}

func (s *service) FileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	file, err := s.Queries.GetFile(ctx, int64(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(file.Content)
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
