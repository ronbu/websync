(ns websync.core-test
  (:require [clojure.test :refer :all]
            [websync.core :refer :all])
  (:require [clj-http.client :as client]
            [clojure.core.async :as async]
            [clj-time.core :as ts]
            [clj-time.coerce :as tc]
            [clojure.java.io :as io]
            [clojure.contrib.java-utils :as utils]
            [clojure.data :as clj-data]))

(defn read-file [{:keys [path]}]
      {:mtime (file-mtime path)
       :path path})

(defn add-tmp [tmp file]
  (assoc file :path (utils/file tmp (:path file))))

(defn rm-tmp [tmp {:keys [path] :as file}]
  (if (nil? path)
             file
             (assoc file :path (clojure.string/replace path tmp ""))))

(defn write-tmp [tmp path mtime & [dir]]
  (let [file (add-tmp tmp {:path path :mtime mtime})
        path (:path file)]
    (if dir
      (doto (utils/file path)
        (.mkdirs)
        (.mkdir)
        (set-mtime mtime))
      (write-file file ""))
    (:mtime (read-file file))))


(deftest write-file-test
  (let [tmp    (java.io.File/createTempFile "websync" "")
        _      (.delete tmp) ; TODO: Create a dir not a file
        dt     (ts/date-time 2008)
        before (ts/minus dt (ts/years 1))
        after  (ts/plus dt (ts/secs 12))]
    (testing "Write File"
      (is (= (write-tmp tmp "a" dt) dt)))
    (testing "Update File"
      (write-tmp tmp "b" dt)
      (is (= (write-tmp tmp "b" after) after)))
    (testing "Do not update older File"
      (write-tmp tmp "c" dt)
      (is (= (write-tmp tmp "c" before) dt)))
    (testing "Do not overwrite directory"
      (write-tmp tmp "d" dt :dir)
      (is (= (write-tmp tmp "d" after) dt)))
    (utils/delete-file-recursively tmp)))
