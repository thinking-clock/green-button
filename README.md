# green-button

Fill out config.yaml

`docker run -v /.../config.yaml:/etc/green-button/config.yaml ghcr.io/thinking-clock/green-button:main`

```
  alectra:
    image: ghcr.io/thinking-clock/green-button:main
    container_name: alectra
    restart: unless-stopped
    volumes:
      - /.../config.yaml:/etc/green-button/config.yaml 
```
