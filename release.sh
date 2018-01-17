#!/usr/bin/env bash

build_and_push() {
	tag="$1"
	while true; do
		read -r -p "Do you really want to push seemethere/unir:$tag? " yn
		case $yn in
			[Yy]* )
				docker build -t "seemethere/unir:$tag" .
				docker push "seemethere/unir:$tag"
				break
				;;
			[Nn]* )
				echo "Exiting..."
				exit
				;;
			* )
				echo "Please answer yes or no."
				;;
		esac
	done
}

main() {
	VERSION=${VERSION:-dev}
	build_and_push "$VERSION"
	build_and_push "latest"
	git tag "v$VERSION"
	git push --tags
}

main
