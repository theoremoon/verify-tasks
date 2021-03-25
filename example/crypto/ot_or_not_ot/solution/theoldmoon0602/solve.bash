echo "pyopyo $HOST hogehoge"
docker-compose build
docker-compose run -e "HOST=${HOST}" solve python solve.py
