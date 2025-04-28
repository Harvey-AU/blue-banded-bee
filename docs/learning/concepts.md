# Learning Concepts

## Environment and Defaults

- `godotenv.Load()` loads the `.env` file into environment variables
- `os.Getenv` reads those environment variables
- `getEnvWithDefault` returns the environment variable or your specified fallback

## Structs and Pointers

- `Config{}` creates a new struct value
- `&Config{}` makes a pointer to that struct
- `*ptr` dereferences a pointer to get its value

## Command-Line Flags

- `flag.String` / `flag.Bool` declare CLI options and return pointers
- `flag.Parse()` reads user input into those pointers

## Data Types

### Basic Types

- **string:** Holds text, e.g. `"8080"`

  ```go
  config.Port = getEnvWithDefault("PORT", "8080")
  ```

- **bool:** Holds true or false, e.g.

  ```go
  if *findLinks {
    // do link-finding
  }
  ```

- **int:** Holds whole numbers, e.g.
  ```go
  config.MaxIdleConns = 10
  ```

### Composite Types

- **slice (`[]T`)**  
  A dynamic list of items of type T, e.g.

  ```go
  fruits := []string{"apple", "banana", "cherry"}
  ```

- **map (`map[K]V`)**  
  A lookup table mapping keys of type K to values of type V, e.g.

  ```go
  data := map[string]string{
    "status": "ok",
    "port":   config.Port,
  }
  json.NewEncoder(w).Encode(data)
  ```
