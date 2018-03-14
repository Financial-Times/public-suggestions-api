var hooks = require('hooks');
var http = require('http');
var fs = require('fs');

const defaultFixtures = './_ft/ersatz-fixtures.yml';

hooks.beforeAll(function(t, done) {
   if(!fs.existsSync(defaultFixtures)){
      console.log('No fixtures found, skipping hook.');
      done();
      return;
   }

   var contents = fs.readFileSync(defaultFixtures, 'utf8');

   var options = {
      host: 'localhost',
      port: '9000',
      path: '/__configure',
      method: 'POST',
      headers: {
         'Content-Type': 'application/x-yaml'
      }
   };

   var req = http.request(options, function(res) {
      res.setEncoding('utf8');
   });

   req.write(contents);
   req.end();
   done();
});


hooks.beforeEach(function (transaction) {
    // see https://github.com/apiaryio/dredd/blob/master/docs/hooks-nodejs.md#modifying-transaction-request-body-prior-to-execution
    // and because of https://github.com/apiaryio/dredd/blob/master/docs/how-it-works.md#swagger-2
    // "By default Dredd tests only responses with 2xx status codes. Responses with other codes are marked as skipped and can be activated in hooks"
    if (transaction.name.startsWith("Internal API > /content/suggest > Suggests annotations > 400")) {
        transaction.skip = false
        transaction.request.body = "wrong_json"
    }
    if (transaction.name.startsWith("Health > /__gtg")) {
        hooks.log("skipping: " + transaction.name);
        transaction.skip = true;
    }
});