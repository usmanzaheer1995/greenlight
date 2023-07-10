create index if not exists movies_title_idx on movies using GIN (to_tsvector('simple', title));
create index if not exists movies_genres_idx on movies using GIN (genres);
