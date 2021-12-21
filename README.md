# Tdispo

Tdispo is a Doodle like web application to record participations to events.

## Development

It requires Go and [Tailwind CLI](https://tailwindcss.com/docs/installation).

### Build and run

```
go generate
go build
./tdispo
```

Then browse [http://localhost:8080](http://localhost:8080).

### Live reload

I simply use this command line with the [entr](http://eradman.com/entrproject/) utility.

```
git ls-files '*.go' '*.html' | entr -crs 'go generate; go build; ./tdispo'
```
