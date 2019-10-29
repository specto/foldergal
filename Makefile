.PHONY: run, clear_cache

run:
	python src/www.py

clear_cache:
	rm -rf cache/*.{jpg,png,JPG,PNG,jfif}
