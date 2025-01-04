package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Video metadata not found!", err)
		return
	}

	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't own this video!", err)
		return
	}

	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse video", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unknown media type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid media type: %v", mediaType), nil)
		return
	}

	tmpFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Upload processing error", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	io.Copy(tmpFile, file)
	tmpFile.Seek(0, io.SeekStart)

	processedFilepath, err := processVideoForFastStart(tmpFile.Name())
	if err != nil {
		fmt.Println(err)
		respondWithError(w, http.StatusInternalServerError, "Error storing file (1)", err)
		return
	}

	processedFile, err := os.Open(processedFilepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error storing file (2)", err)
		return
	}
	defer processedFile.Close()

	aspectRatio, err := getVideoAspectRatio(processedFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Upload processing error (3)", err)
		return
	}

	var prefix string
	if aspectRatio == "16:9" {
		prefix = "landscape"
	} else if aspectRatio == "9:16" {
		prefix = "portrait"
	} else {
		prefix = "other"
	}

	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error storing file", err)
		return
	}
	extension := strings.Split(mediaType, "/")[1]
	filename := path.Join(prefix, base64.RawURLEncoding.EncodeToString(bytes)+"."+extension)

	inputOpts := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filename,
		Body:        processedFile,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(context.Background(), &inputOpts)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Upload processing error", err)
		return
	}

	videoURL := fmt.Sprintf("https://%v/%v", cfg.s3CfDistribution, filename)
	// videoURL := fmt.Sprintf("%v,%v", cfg.s3Bucket, filename)

	video := database.Video{
		ID:           videoID,
		CreatedAt:    metadata.CreatedAt,
		UpdatedAt:    time.Now(),
		ThumbnailURL: metadata.ThumbnailURL,
		VideoURL:     &videoURL,
		CreateVideoParams: database.CreateVideoParams{
			Title:       metadata.Title,
			Description: metadata.Description,
			UserID:      metadata.UserID,
		},
	}
	cfg.db.UpdateVideo(video)

	// video, err = cfg.dbVideoToSignedVideo(video)
	// if err != nil {
	// 	respondWithError(w, http.StatusInternalServerError, "Error generating video link", err)
	// 	return
	// }

	respondWithJSON(w, http.StatusOK, video)
}

// func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
// 	params := s3.GetObjectInput{
// 		Bucket: &bucket,
// 		Key:    &key,
// 	}
// 	presignClient := s3.NewPresignClient(s3Client)
//
// 	presignRequest, err := presignClient.PresignGetObject(context.Background(), &params, s3.WithPresignExpires(expireTime))
// 	if err != nil {
// 		return "", err
// 	}
//
// 	return presignRequest.URL, nil
// }
//
// func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
// 	if video.VideoURL == nil {
// 		return video, nil
// 	}
// 	parts := strings.Split(*video.VideoURL, ",")
// 	bucket, key := parts[0], parts[1]
//
// 	presignedUrl, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Duration(10)*time.Minute)
// 	if err != nil {
// 		return video, err
// 	}
//
// 	video.VideoURL = &presignedUrl
//
// 	return video, nil
//
// }
