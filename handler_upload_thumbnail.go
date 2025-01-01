package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse thumbnail", err)
		return
	}

	mediaType := header.Header.Get("Content-Type")
	imagedata, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read image", err)
		return
	}

	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Video metadata not found!", err)
		return
	}

	fmt.Println(metadata.UserID)
	fmt.Println(userID)
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't own this video!", err)
		return
	}

	videoThumbnails[videoID] = thumbnail{
		data:      imagedata,
		mediaType: mediaType,
	}

	thumbnailURL := fmt.Sprintf("http://localhost:8091/api/thumbnails/%v", videoID)
	video := database.Video{
		ID:           videoID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		ThumbnailURL: &thumbnailURL,
		VideoURL:     metadata.VideoURL,
		CreateVideoParams: database.CreateVideoParams{
			Title:       metadata.Title,
			Description: metadata.Description,
			UserID:      metadata.UserID,
		},
	}
	cfg.db.UpdateVideo(video)

	respondWithJSON(w, http.StatusOK, video)
}
