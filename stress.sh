for (( i = 1; i <= 20; i++)); do
  /tmp/bin/gobench -u http://localhost:8080/user/$((1 + $RANDOM % 5339)) -k=true -c $((1 + $RANDOM % 70)) -t $((30 + $RANDOM % 300)) &
done