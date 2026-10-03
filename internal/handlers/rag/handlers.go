package rag

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"TheRAGService/internal/httpx"
	"TheRAGService/internal/middlewares"
	"TheRAGService/internal/models"
	"TheRAGService/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func CreateNewCollection(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	claims, ok := middlewares.ClaimsFromContext(ctx)

	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.CreateNewCollectionReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		httpx.WriteError(
			w, http.StatusBadRequest,
			"Invalid JSON",
		)
		return
	}

	if len(req.CollectionName) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "collection name is required")
		return
	}

	if len(req.CollectionName) > 100 {
		httpx.WriteError(
			w, http.StatusBadRequest,
			"collection name is too long",
		)
		return
	}

	_, err = store.DB.Exec(
		ctx,
		`
		INSERT INTO collections (
			user_id,collection_name, description
		)
		VALUES ($1,$2,$3)
		`,
		claims.UserID, req.CollectionName, req.Description,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httpx.WriteError(w, http.StatusConflict, "collection name already exists")
			return
		}

		log.Println("[ERROR] Insert collection error: ", err)
		httpx.WriteError(w, http.StatusInternalServerError, "Database error")
		return
	}

	httpx.WriteJSON(
		w, http.StatusCreated,
		map[string]any{
			"message": "Collection created",
		},
	)
}

func MyCollections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := middlewares.ClaimsFromContext(ctx)

	if !ok {
		httpx.WriteError(
			w, http.StatusUnauthorized,
			"Unauthorized",
		)
		return
	}

	collections := []models.Collections{}

	rows, err := store.DB.Query(
		ctx,
		`
		SELECT id, collection_name, description, created_at
		FROM collections
		WHERE user_id=$1
		`,
		claims.UserID,
	)

	if err != nil {
		httpx.WriteError(
			w, http.StatusInternalServerError,
			"Database error",
		)
		return
	}

	defer rows.Close()

	for rows.Next() {
		var collection models.Collections

		err := rows.Scan(
			&collection.CollectionID,
			&collection.CollectionName,
			&collection.CollectionDesc,
			&collection.CreatedAt,
		)

		if err != nil {
			httpx.WriteError(
				w, http.StatusInternalServerError,
				"Failed to scan collections",
			)
			return
		}

		collections = append(collections, collection)
	}

	if err := rows.Err(); err != nil {
		httpx.WriteError(
			w, http.StatusInternalServerError,
			"failed to iterate collections",
		)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"collections": collections,
	})

}

func DeleteCollection(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	claims, ok := middlewares.ClaimsFromContext(ctx)

	if !ok {
		httpx.WriteError(
			w, http.StatusUnauthorized,
			"Unauthorized",
		)
		return
	}

	collection_id, err := strconv.ParseInt(
		chi.URLParam(r, "collection_id"), 10, 64,
	)

	if err != nil {
		httpx.WriteError(
			w, http.StatusBadRequest,
			"Invalid Collection ID",
		)
		return
	}

	// Delete is scoped by user_id so a user can never delete
	// another user's collection by guessing its ID.
	tag, err := store.DB.Exec(
		ctx,
		`
		DELETE FROM collections
		WHERE id=$1 AND user_id=$2
		`,
		collection_id,
		claims.UserID,
	)

	if err != nil {
		log.Println("[ERROR] Delete collection error: ", err)
		httpx.WriteError(
			w, http.StatusInternalServerError,
			"Database error",
		)
		return
	}

	if tag.RowsAffected() == 0 {
		httpx.WriteError(
			w, http.StatusNotFound,
			"Collection not found",
		)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Collection deleted",
	})
}
