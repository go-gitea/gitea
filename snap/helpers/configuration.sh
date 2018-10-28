#!/bin/bash
if snapctl get gitea.snap.custom; then
  cdir=$(snapctl get gitea.snap.custom)
else
  cdir=$SNAP_COMMON
fi

cfg="$cdir/conf/app.ini"
bak="$cdir/conf/app.ini.bak-$(date -Ins)"
basecfg="$SNAP/snap/helpers/app.ini"
smp="$SNAP/gitea/custom/conf/app.ini.sample"

function toSnap() {
OIFS=$IFS
IFS='
'
  category="none"
  src="$cfg"
  [[ "$1" = "init" ]] && src="$smp"
  [[ "$1" = "snap" ]] && src="$basecfg"

  for l in $(sed 's_;\([A-Z]*\)_\1_g' $src | grep -v -e '^;' -e '^$'); do
    if echo $l | grep -q '^[[]'; then
      category=$(CatToSnap "$l")
    elif echo $l | grep -q '^[A-Z]'; then
      option=$(OptToSnap "$l")
      value=$(ValToSnap "$l")
      if [[ $category = "none" ]]; then
        snapctl set "$option=$value"
      else
        snapctl set "$category.$option=$value"
      fi
    fi
  done
IFS=$OIFS
}

function toIni() {
OIFS=$IFS
IFS='
'
  category="none"; option="none"; catUnset=true
  src=$smp
  [[ -f $cfg ]] && src="$cfg"
  tmpIni="$cfg.tmp"
  [[ -f $src ]] && cp "$src" "$tmpIni"
  cp $tmpIni $bak
  echo '' > $cfg
  for l in $(grep -v -e '^;' -e '^$' $tmpIni); do
    if echo $l | grep -q '^[[]'; then
      category=$(CatToSnap "$l")
      catUnset=true
    elif echo $l | grep -q '^[A-Z]'; then
      option=$(OptToSnap "$l")
      if [[ $category = "none" ]]; then
        value=$(snapctl get $option)
        echo $(OptToIni "$option") = $value >> $cfg
      else
        value=$(snapctl get $category.$option)
        if $catUnset; then
          echo "" >> $cfg
          echo "[$(CatToIni "$category")]" >> $cfg
          catUnset=false
        fi
        echo $(OptToIni "$option") = $value >> $cfg
      fi
    fi
  done;
  IFS=$OIFS
}

function CatToSnap {
  ret=$(echo "$1"                             \
         | grep -oP '[A-Za-z0-9._]+'          \
         | sed 's|\.|-|g'                     \
         | sed 's|_|99|g')
  echo $ret
}
function OptToSnap {
  ret=$(echo "$1"                             \
         | grep -oP '^[A-Z_]+'                \
         | tr '[:upper:]' '[:lower:]'         \
         | sed 's|_|-|g')
  echo $ret
}
function ValToSnap {
  ret=$(echo "$1"                             \
         | grep -oP '=.*$'                    \
         | sed 's_^= __g'                     \
         | sed 's_^=__g'                      \
         | sed "s|SNAP_DIR_DATA|$SDATA|g"     \
         | sed "s|SNAP_DIR_COMMON|$SCOMMON|g" \
         | sed 's|{}||g')
  echo $ret
}

function CatToIni {
  ret=$(echo "$1"                             \
         | sed 's|-|.|g'                      \
         | sed 's|\ |_|g'                     \
         | sed 's|99|_|g')
  echo $ret
}
function OptToIni {
  ret=$(echo "$1"                             \
         | tr '[:lower:]' '[:upper:]'         \
         | sed 's|-|_|g')
  echo $ret
}

[[ "$1" = "configure" ]]             \
  && toIni                           \
  && exit 0

[[ "$1" = "install" ]]               \
  && echo "Initial Configuration..." \
  && mkdir -p $SNAP_COMMON/conf      \
  && toSnap init                     \
  && toSnap snap                     \
  && toIni sample                    \
  && exit 0

[[ "$1" = "save" ]]                  \
  && echo "Saving current config..." \
  && toSnap                          \
  && exit 0
