function grader(payload) {
    var result = 0

    _.each(_.get(payload, ['data', 'predictions'], []), function(value, key) {
        if (value["score"] > 0.3) {
            result = 1
        }
    })

    return result
}

grader({'data':{'predictions':{'dummy':{'score':0}}}})

grader({
    "code": 0,
    "message": "",
    "data": {
        "predictions": {
            "d6f1db8f-48e2-4cbd-add0-357643174669": {
                "score": 0.9007511138916016,
                "labelName": "Flat Coated Retriever",
                "labelIndex": 4,
                "defectId": 2398650,
                "coordinates": {"xmin": 111, "ymin": 31, "xmax": 643, "ymax": 545}
            }
        },
        "type": "ObjectDetectionPrediction",
        "latency": {
            "preprocess_s": 0.0012102127075195312,
            "infer_s": 0.5102131366729736,
            "postprocess_s": 1.049041748046875e-05,
            "serialize_s": 0.00061798095703125
        }
    }
})