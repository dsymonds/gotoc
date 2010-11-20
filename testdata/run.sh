#!/bin/bash -e

cd $(dirname $0)

MAX=100
GOTOC=../gotoc
PROTOCMP=./protocmp

failures=0
for ((i=1; $i <= $MAX; i=$((i+1)))); do
  if [ ! -f $i.proto ]; then continue; fi
  echo "---[ Test $i ]---" 1>&2

  $GOTOC --descriptor_only $i.proto > $i.actual
  $PROTOCMP $i.expected $i.actual || {
    echo "==> FAILED" 1>&2
    failures=$(($failures + 1))
  }
done

echo "----------" 1>&2
if [ $failures -eq 0 ]; then
  echo "All OK" 1>&2
else
  echo "$failures test failure(s)" 1>&2
fi
echo "----------" 1>&2
exit $failures
