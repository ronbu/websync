(ns websync.core
  (:require [clj-http.client :as client]
            [clojure.core.async :as async]
            [clj-time.core :as ts]
            [clj-time.coerce :as tc]
            [clojure.java.io :as io]
            [clojure.contrib.java-utils :as utils]
            [clojure.data :as clj-data])
  (:import java.io.File)
  (:require [clojure.test :refer :all]))

(defn file-mtime [path]
  (tc/from-long (.lastModified (io/file path))))

(defn set-mtime [file ts]
  (.setLastModified file (tc/to-long ts)))

(defn write-file [{:keys [mtime path]} stream]
    (let  [file (io/file path)
           _ (.isDirectory file)]
      (when (and (ts/after? mtime (file-mtime path))
                 (not (.isDirectory file)))
        (io/make-parents path)
        (.createNewFile file)
        (io/copy stream (io/output-stream file))
        (set-mtime file mtime))))
