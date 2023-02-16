CREATE TABLE IF NOT EXISTS "user" (
    id uuid default gen_random_uuid() PRIMARY KEY,
    "login" varchar(100) not null,
    password_hash varchar(100) not null,
    CONSTRAINT unique_login UNIQUE(login)
);
CREATE TABLE IF NOT EXISTS "order" (
    id uuid default gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    status varchar(20),
    amount real,
    external_id varchar(100) NOT NULL,
    registered_at timestamp default now() NOT NULL,
    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES "user"(id)
);
CREATE TABLE IF NOT EXISTS withdrawal (
    id uuid default gen_random_uuid()  PRIMARY KEY,
    user_id uuid NOT NULL,
    amount real NOT NULL,
    external_id varchar(100) NOT NULL,
    registered_at timestamp default now() NOT NULL ,
    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES "user"(id)
);