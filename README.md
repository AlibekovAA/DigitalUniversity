# Проект

Этот проект представляет собой приложение цифрового вуза в рамках хакатона VK & Max.

Представленное решение оказывает помощь в повседневной жизни студента. Основной решаемой задачей является реализация базовых потребностей студентов и преподавателей в удобном формате. С помощью данного чат-бота, интегрированного в месенджер  Max, студент может быстро и удобно просматривать расписание (с навигацией по дням недели), свою посещаемость и успеваемость по дисциплинам.Реализована возможность получения уведомлений при выставлении оценки или пропуска. Со стороны преподавателя есть возможность также просматривать расписание, выставлять оценки и посещаемость студентам. Весь предлагаем интерфейс максимально интуитивно понятный и простой.

В проекте есть распределение ролей:
* Администратор
* Преподаватель
* Студент

Перед началом работы необходимо загрузить данные администратора в бд (указать нужную роль). Затем администратор должен подгрузить данные о студентах, преподавателях и расписании в виде csv-файлов (пример заполнения находится в `/docs`)

## Общая архитектура системы

## Запуск проекта

Для запуска проекта необходимо выполнить следующие шаги:

#### 1. Перемнные окружения
Необходимо заполнить файл с переменными окружения `.env` и поместить его в корень проекта.Подробный пример заполнения файла и описание каждой переменной можно посмотреть в `.env.example`.
    
    `LOG_LEVEL=0` - Уровень логирования

    `LOG_DIR=../logs` - Директория для файлов логов

    `DATABASE_URI=postgres://user:password@localhost:5432/dbname?sslmode=disable` - URI для связи с базой данных

    `MAX_TOKEN`=your_bot_token_here - Токен бота в Max

    `POSTGRES_USER=user` - Имя пользователя в PostgresDB

    `POSTGRES_PASSWORD=password` - Пароль в PostgresDB

    `POSTGRES_DB=dbname` - Название бд в PostgresDB


#### 2. Способ запуска
Для запуска можно использовать инструкции из `Makefile`

* `make up` - запустить проект
* `make down` - остановить проект
* `make reup` - перезапуск проекта
* `mape clean` - очистка окружения

Также осуществлять запуск можно с помощью `Docker-compose`

* `docker-compose up --build` - запустить проект
* `docker-compose down -v` - остановить проект

Очистка окружения:
```bash
docker stop $(docker ps -a -q)
docker rm $(docker ps -a -q)
docker rmi $(docker images -q)
docker system prune -a --volumes -f
```


#### 3. Хранение данных
Для хранения информации используется СУБД Postgres.
При запуске проекта происходит создание таблиц (полный код находится в `db/initdb.sql`). В конце данного файла есть пример заполнения данных администратора.
##### Структура базы данных
###### Таблица пользователей (users)

```sql
CREATE TABLE IF NOT EXISTS users (
    user_id SERIAL PRIMARY KEY,
    "name" VARCHAR(255) NOT NULL,
    usermax_id BIGINT UNIQUE,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    role_id INT REFERENCES roles(role_id),
    group_id INT REFERENCES groups(group_id)
);
```

###### Таблица ролей (roles)

```sql
CREATE TABLE IF NOT EXISTS roles (
    role_id SERIAL PRIMARY KEY,
    role_name VARCHAR(50) UNIQUE NOT NULL
);
```

###### Таблица дисциплин (subjects)

```sql
CREATE TABLE IF NOT EXISTS subjects (
    subject_id SERIAL PRIMARY KEY,
    subject_name VARCHAR(255) NOT NULL,
    teacher_id INT NOT NULL REFERENCES users(user_id)
);
```

###### Таблица групп (groups)

```sql
CREATE TABLE IF NOT EXISTS groups (
    group_id SERIAL PRIMARY KEY,
    group_name VARCHAR(100) UNIQUE NOT NULL
);
```

###### Таблица связи групп и дисциплин (groups_subjects)

```sql
CREATE TABLE IF NOT EXISTS groups_subjects (
    group_id INT NOT NULL REFERENCES groups(group_id),
    subject_id INT NOT NULL REFERENCES subjects(subject_id),
    PRIMARY KEY (group_id, subject_id)
);
```

###### Таблица типов занятий (lesson_types)

```sql
CREATE TABLE IF NOT EXISTS lesson_types (
    lesson_type_id SERIAL PRIMARY KEY,
    type_name VARCHAR(50) UNIQUE NOT NULL
);

```

###### Таблица расписания (schedule)

```sql
CREATE TABLE IF NOT EXISTS schedule (
    schedule_id SERIAL PRIMARY KEY,
    weekday SMALLINT NOT NULL CHECK (
        weekday BETWEEN 1 AND 7
    ),
    class_room VARCHAR(100),
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    subject_id INT NOT NULL REFERENCES subjects(subject_id),
    teacher_id INT NOT NULL REFERENCES users(user_id),
    group_id INT NOT NULL REFERENCES groups(group_id),
    lesson_type_id INT NOT NULL REFERENCES lesson_types(lesson_type_id)
);
```

###### Таблица оценок (grades)

```sql
CREATE TABLE IF NOT EXISTS grades (
    grade_id SERIAL PRIMARY KEY,
    student_id INT NOT NULL REFERENCES users(user_id),
    teacher_id INT NOT NULL REFERENCES users(user_id),
    subject_id INT NOT NULL REFERENCES subjects(subject_id),
    schedule_id INT NOT NULL REFERENCES schedule(schedule_id),
    grade_value INT NOT NULL CHECK (
        grade_value BETWEEN 0 AND 5
    ),
    grade_date TIMESTAMP DEFAULT NOW()
);
```

###### Таблица посещаемости (attendance)

```sql
CREATE TABLE IF NOT EXISTS attendance (
    attendance_id SERIAL PRIMARY KEY,
    student_id INT NOT NULL REFERENCES users(user_id),
    schedule_id INT NOT NULL REFERENCES schedule(schedule_id),
    attended BOOLEAN NOT NULL,
    mark_time TIMESTAMP DEFAULT NOW()
);
```

###### Таблица материалов (materials)

```sql
CREATE TABLE IF NOT EXISTS materials (
    material_id SERIAL PRIMARY KEY,
    subject_id INT NOT NULL REFERENCES subjects(subject_id),
    file_url TEXT NOT NULL,
    uploaded_at TIMESTAMP DEFAULT NOW()
);
```

###### Взаимосвязи между таблицами

![alt text](screenshots/db.jpg)

#### 4. Разработка

##### Backend

Общая структура
```
├── application
│   └── application.go
├── backend.yml
├── config
│   └── config.go
├── database
│   ├── database.go
│   ├── models.go
│   └── repository.go
├── Dockerfile
├── go.mod
├── go.sum
├── logger
│   └── logger.go
├── main.go
├── maxAPI
│   ├── attendance.go
│   ├── bot.go
│   ├── handlers.go
│   ├── keyboard.go
│   ├── message.go
│   ├── schedule.go
│   ├── student_grades.go
│   ├── teacher_grades.go
│   └── utils.go
├── services
│   ├── importer.go
│   └── validator.go
```
- Расположен в `src/`
- Используются Go-модули (`go.mod`, `go.sum`), точка входа — `main.go`.
- Конфигурация и сборка: `backend.yml`, `Dockerfile`.
- Модульная структура:
  - `src/application/` — инициализация и запуск приложения
    - `application.go`
  - `src/config/` — загрузка и управление конфигурацией
    - `config.go`
  - `src/database/` — подключение к БД и репозиторий
    - `database.go`
    - `models.go` — ORM-модели
    - `repository.go` — методы доступа к данным
  - `src/logger/` — кастомный логгер
    - `logger.go`
  - `src/maxAPI/` — интеграция с Max
    - `bot.go` — инициализация бота
    - `handlers.go` — обработчики команд и callback-ов
    - `keyboard.go` — генерация клавиатур
    - `schedule.go` — работа с расписанием через бота
    - `attendance.go` — работа с посещаемостью
    - `message.go` — работа с отправкой сообщений
    - `student_grades.go` — работа с оценками для студента
    - `teachers_grades.go` — работа с оценками для преподавателя
    - `utils.go`  — вспомогательные методы
  - `src/services/` — бизнес-логика и вспомогательные сервисы
    - `importer.go` — импорт данных
    - `validator.go` — валидация входных данных


