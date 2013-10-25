(ns websync.core
  (:require [clojure.core.async :as a]
            [clj-time.core :as ts]
            [clj-time.coerce :as tc]
            [clojure.java.io :as io]
            [clojure.contrib.java-utils :as utils]
            [clojure.data :as clj-data]
            [org.httpkit.client :as client]
            [net.cgrand.enlive-html :as html])
  (:import java.io.File)
  (:require [clojure.test :refer :all]))

(defn file-mtime [path]
  (tc/from-long (.lastModified (io/file path))))

(defn set-mtime [file ts]
  (.setLastModified file (tc/to-long ts)))

(defn write-file [{:keys [mtime path]} stream]
    (let  [file (io/file path)]
      (if (.isDirectory file)
        (str path ": is a directory")
        (when (and (ts/after? mtime (file-mtime path)))
          (io/make-parents path)
          (.createNewFile file)
          (io/copy stream (io/output-stream file))
          (set-mtime file mtime)
          nil))))

(def zdfHost "http://www.zdf.de")
(def zdfMediathek (str zdfHost "/ZDFmediathek/"))
(def zdfDay (str zdfMediathek
               "hauptnavigation/sendung-verpasst/day%d?flash=off"))

(defn url [&[url relative]]
  (java.net.URL. (io/as-url url) (or relative "")))

(url "http://w.com/holla" "haha") ; TODO: fix this bug

(defn zdf-url [href] (url zdfHost href))

(defn get-href [a] (get-in a [:attrs :href]))

(defn zdf-post [link]
  (let [posttree (html/html-resource (zdf-url (get-href link)))
        videoname (first (:content (first (html/select
                                           posttree
                                           [:h1.beitragHeadline]))))
        videolink (last (html/select posttree [:ul.dslChoice :a]))
        videourl (zdf-url (get-href videolink))]
   {:name (str videoname ".mov") :url videourl}))

(defn zdf-post-links [url]
  (html/select (html/html-resource (io/as-url (format zdfDay url)))
               [:div.beitragListe :div.image :a]))

(defn zdf [file]
  (map zdf-post (flatten (map zdf-post-links (range 1)))))
