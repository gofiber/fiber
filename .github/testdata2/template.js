// This is a single-line comment
var x = 10; // This is also a single-line comment

/*
This is a multi-line comment
It can span across multiple lines
*/

// Function that returns the factorial of a number
function factorial(n) {
    if (n <= 1) {
        return 1;
    };
    return n * factorial(n - 1); // Recursive call
};

// Anonymous function assigned to a variable
var double = function(x) {
    return x * 2;
};

// Array of numbers
var numbers = [1, 2, 3, 4, 5];

// Loop through the array and double each number
for (var i = 0; i < numbers.length; i++) {
    numbers[i] = double(numbers[i]);
};

// Object with properties and methods
var person = {
    name: "John",
    age: 30,
    greet: function() {
        console.log("Hello, my name is " + this.name + " and I'm " + this.age + " years old.");
    }
};

// Call the greet method
person.greet();
